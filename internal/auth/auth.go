package auth

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"walrus/internal/apperr"
	"walrus/internal/db"
)

const (
	CookieName     = "walrus_token"
	ContextUserKey = "auth_user"
	TokenTTL       = 7 * 24 * time.Hour
)

var (
	ErrInvalidCredentials = apperr.New(apperr.CodeInvalidCredentials, "invalid username or password")
	ErrAccountDisabled    = apperr.New(apperr.CodeAccountDisabled, "account is disabled")
	ErrRateLimited        = apperr.New(apperr.CodeRateLimited, "too many failed login attempts, please try again later")
)

type Claims struct {
	UserID   int64   `json:"uid"`
	Username string  `json:"usr"`
	Role     db.Role `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	store        *db.Store
	secret       []byte
	cookieSecure bool

	rlMu     sync.Mutex
	attempts map[string][]time.Time
}

func NewService(store *db.Store, secret string, cookieSecure bool) *Service {
	return &Service{
		store:        store,
		secret:       []byte(secret),
		cookieSecure: cookieSecure,
		attempts:     make(map[string][]time.Time),
	}
}

func (s *Service) HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

func (s *Service) CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func (s *Service) IssueToken(u *db.User) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.secret)
}

func (s *Service) ParseToken(raw string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

func (s *Service) SetAuthCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, token, int(TokenTTL.Seconds()), "/", "", s.cookieSecure, true)
}

func (s *Service) ClearAuthCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, "", -1, "/", "", s.cookieSecure, true)
}

// Login validates credentials with rate limiting (max 8 failed attempts per 5 minutes per key).
func (s *Service) Login(username, password, clientIP string) (*db.User, string, error) {
	key := clientIP + "|" + strings.ToLower(strings.TrimSpace(username))
	if s.isRateLimited(key) {
		return nil, "", ErrRateLimited
	}

	u, err := s.store.GetUserByUsername(strings.TrimSpace(username))
	if err != nil {
		s.recordFailure(key)
		return nil, "", ErrInvalidCredentials
	}
	if u.Disabled {
		return nil, "", ErrAccountDisabled
	}
	if !s.CheckPassword(u.PasswordHash, password) {
		s.recordFailure(key)
		return nil, "", ErrInvalidCredentials
	}

	s.clearFailures(key)
	tok, err := s.IssueToken(u)
	if err != nil {
		return nil, "", err
	}
	return u, tok, nil
}

func (s *Service) isRateLimited(key string) bool {
	s.rlMu.Lock()
	defer s.rlMu.Unlock()
	cutoff := time.Now().Add(-5 * time.Minute)
	list := s.attempts[key]
	filtered := list[:0]
	for _, ts := range list {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	s.attempts[key] = filtered
	return len(filtered) >= 8
}

func (s *Service) recordFailure(key string) {
	s.rlMu.Lock()
	defer s.rlMu.Unlock()
	s.attempts[key] = append(s.attempts[key], time.Now())
}

func (s *Service) clearFailures(key string) {
	s.rlMu.Lock()
	defer s.rlMu.Unlock()
	delete(s.attempts, key)
}

// ExtractToken retrieves JWT from cookie, Authorization header, or query param (for WebSocket).
func ExtractToken(c *gin.Context) string {
	if tok, err := c.Cookie(CookieName); err == nil && tok != "" {
		return tok
	}
	if hdr := c.GetHeader("Authorization"); strings.HasPrefix(hdr, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(hdr, "Bearer "))
	}
	if q := c.Query("token"); q != "" {
		return q
	}
	return ""
}

func (s *Service) AuthenticateRequest(c *gin.Context) (*db.User, error) {
	raw := ExtractToken(c)
	if raw == "" {
		return nil, errors.New("missing token")
	}
	claims, err := s.ParseToken(raw)
	if err != nil {
		return nil, err
	}
	u, err := s.store.GetUserByID(claims.UserID)
	if err != nil || u.Disabled {
		return nil, errors.New("user unavailable")
	}
	return u, nil
}

func (s *Service) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := s.AuthenticateRequest(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apperr.CodeBody(apperr.CodeUnauthorized, "unauthorized", nil))
			return
		}
		c.Set(ContextUserKey, u)
		c.Next()
	}
}

func (s *Service) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil || u.Role != db.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, apperr.CodeBody(apperr.CodeAdminRoleRequired, "admin role required", nil))
			return
		}
		c.Next()
	}
}

// RequireAnyOperator allows users who hold operator (or higher) access on at
// least one cluster; used for global resources shared by cluster operators.
func (s *Service) RequireAnyOperator() gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, err := s.store.HasOperatorAccess(CurrentUser(c))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, apperr.CodeBody(apperr.CodePermissionLookupFailed, "permission lookup failed", nil))
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, apperr.CodeBody(apperr.CodeInsufficientClusterPerm, "insufficient cluster permission", nil))
			return
		}
		c.Next()
	}
}

// RequireClusterRole checks user's effective role on :id cluster path parameter.
func (s *Service) RequireClusterRole(minRole db.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apperr.CodeBody(apperr.CodeUnauthorized, "unauthorized", nil))
			return
		}
		clusterID := c.Param("id")
		eff, err := s.store.EffectiveClusterRole(u, clusterID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, apperr.CodeBody(apperr.CodePermissionLookupFailed, "permission lookup failed", nil))
			return
		}
		if eff.Rank() < minRole.Rank() {
			c.AbortWithStatusJSON(http.StatusForbidden, apperr.CodeBody(apperr.CodeInsufficientClusterPerm, "insufficient cluster permission", nil))
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) *db.User {
	val, ok := c.Get(ContextUserKey)
	if !ok {
		return nil
	}
	u, _ := val.(*db.User)
	return u
}
