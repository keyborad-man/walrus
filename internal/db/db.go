package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"walrus/internal/apperr"
)

var ErrNotFound = apperr.New(apperr.CodeNotFound, "record not found")

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles concurrent writers best with a single pooled connection in Go.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: conn}
	if err := s.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'viewer',
			disabled INTEGER NOT NULL DEFAULT 0,
			locale TEXT NOT NULL DEFAULT 'zh-CN',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS user_cluster_permissions (
			user_id INTEGER NOT NULL,
			cluster_id TEXT NOT NULL,
			role TEXT NOT NULL,
			PRIMARY KEY (user_id, cluster_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS clusters_meta (
			cluster_id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			caves_enabled INTEGER NOT NULL DEFAULT 1,
			mod_sync_enabled INTEGER NOT NULL DEFAULT 1,
			auto_restart INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cluster_id TEXT NOT NULL,
			filename TEXT NOT NULL,
			size_bytes INTEGER NOT NULL DEFAULT 0,
			trigger_type TEXT NOT NULL DEFAULT 'manual',
			note TEXT NOT NULL DEFAULT '',
			created_by TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_backups_cluster ON backups(cluster_id, created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			target TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS system_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		// Future-ready tables for Phase 2 & 3
		`CREATE TABLE IF NOT EXISTS player_history (
			ku_id TEXT PRIMARY KEY,
			last_name TEXT NOT NULL DEFAULT '',
			last_cluster_id TEXT NOT NULL DEFAULT '',
			last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS command_snippets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			shard TEXT NOT NULL DEFAULT 'master',
			command TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'custom',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cluster_id TEXT NOT NULL DEFAULT '*',
			name TEXT NOT NULL,
			task_type TEXT NOT NULL,
			cron_expr TEXT NOT NULL,
			params_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			last_run_at DATETIME,
			last_status TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS webhooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			template TEXT NOT NULL DEFAULT 'generic',
			url TEXT NOT NULL,
			secret TEXT NOT NULL DEFAULT '',
			events_json TEXT NOT NULL DEFAULT '[]',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			webhook_id INTEGER NOT NULL,
			event TEXT NOT NULL,
			success INTEGER NOT NULL DEFAULT 0,
			status_code INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created ON webhook_deliveries(created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS mod_cache (
			mod_id TEXT PRIMARY KEY,
			workshop_id TEXT NOT NULL DEFAULT '',
			is_local INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL DEFAULT '',
			author TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			preview_url TEXT NOT NULL DEFAULT '',
			client_only INTEGER NOT NULL DEFAULT 0,
			server_tags TEXT NOT NULL DEFAULT '',
			time_updated INTEGER NOT NULL DEFAULT 0,
			last_synced_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}

	// Ensure default system settings exist.
	defaults := map[string]string{
		"port_pool_start":       "10990",
		"port_pool_end":         "11060",
		"default_cluster_token": "",
		"steam_api_key":         "",
		"dst_branch":            "public",
		"dst_arch":              "64",
		"auto_update_mode":      "manual",
		"backup_max_retain":     "20",
	}
	for k, v := range defaults {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO system_settings(key, value) VALUES (?, ?)`, k, v); err != nil {
			return err
		}
	}
	return nil
}

// --- Settings ---

func (s *Store) GetSetting(key string) (string, error) {
	var val string
	err := s.db.QueryRow(`SELECT value FROM system_settings WHERE key = ?`, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return val, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO system_settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (s *Store) EnsureJWTSecret(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override), nil
	}
	existing, err := s.GetSetting("jwt_secret")
	if err == nil && existing != "" {
		return existing, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(buf)
	if err := s.SetSetting("jwt_secret", secret); err != nil {
		return "", err
	}
	return secret, nil
}

func (s *Store) GetSystemSettings(includeSecrets bool) (*SystemSettings, error) {
	rows, err := s.db.Query(`SELECT key, value FROM system_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	kv := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		kv[k] = v
	}

	parseInt := func(k string, def int) int {
		if n, err := strconv.Atoi(kv[k]); err == nil && n > 0 {
			return n
		}
		return def
	}

	rawToken := kv["default_cluster_token"]
	rawSteam := kv["steam_api_key"]
	st := &SystemSettings{
		PortPoolStart:   parseInt("port_pool_start", 10990),
		PortPoolEnd:     parseInt("port_pool_end", 11060),
		HasDefaultToken: rawToken != "",
		HasSteamAPIKey:  rawSteam != "",
		DSTBranch:       nonEmpty(kv["dst_branch"], "public"),
		DSTArch:         nonEmpty(kv["dst_arch"], "64"),
		AutoUpdateMode:  nonEmpty(kv["auto_update_mode"], "manual"),
		BackupMaxRetain: parseInt("backup_max_retain", 20),
	}
	if includeSecrets {
		st.DefaultClusterToken = rawToken
		st.SteamAPIKey = rawSteam
	} else {
		st.DefaultClusterToken = MaskSecret(rawToken)
		st.SteamAPIKey = MaskSecret(rawSteam)
	}
	return st, nil
}

func MaskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "••••••••" + s[len(s)-4:]
}

func nonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// --- Users & Permissions ---

func (s *Store) CountUsers() (int, error) {
	var c int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&c)
	return c, err
}

func (s *Store) CreateUser(username, passwordHash string, role Role, locale string) (*User, error) {
	if locale == "" {
		locale = "zh-CN"
	}
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO users(username, password_hash, role, disabled, locale, created_at, updated_at) VALUES (?, ?, ?, 0, ?, ?, ?)`,
		username, passwordHash, string(role), locale, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUserByID(id)
}

func (s *Store) GetUserByID(id int64) (*User, error) {
	u := &User{}
	var role string
	var disabled int
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, disabled, locale, created_at, updated_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &disabled, &u.Locale, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Role = Role(role)
	u.Disabled = disabled != 0
	return u, nil
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	var role string
	var disabled int
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, disabled, locale, created_at, updated_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &disabled, &u.Locale, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Role = Role(role)
	u.Disabled = disabled != 0
	return u, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, role, disabled, locale, created_at, updated_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []User
	for rows.Next() {
		var u User
		var role string
		var disabled int
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &disabled, &u.Locale, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.Role = Role(role)
		u.Disabled = disabled != 0
		list = append(list, u)
	}
	return list, nil
}

func (s *Store) UpdateUser(id int64, role Role, disabled bool, passwordHash string, locale string) error {
	now := time.Now().UTC()
	dis := 0
	if disabled {
		dis = 1
	}
	if passwordHash != "" {
		_, err := s.db.Exec(
			`UPDATE users SET role = ?, disabled = ?, password_hash = ?, locale = COALESCE(NULLIF(?, ''), locale), updated_at = ? WHERE id = ?`,
			string(role), dis, passwordHash, locale, now, id,
		)
		return err
	}
	_, err := s.db.Exec(
		`UPDATE users SET role = ?, disabled = ?, locale = COALESCE(NULLIF(?, ''), locale), updated_at = ? WHERE id = ?`,
		string(role), dis, locale, now, id,
	)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) GetUserClusterPermissions(userID int64) ([]UserClusterPermission, error) {
	rows, err := s.db.Query(`SELECT user_id, cluster_id, role FROM user_cluster_permissions WHERE user_id = ? ORDER BY cluster_id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []UserClusterPermission
	for rows.Next() {
		var p UserClusterPermission
		var role string
		if err := rows.Scan(&p.UserID, &p.ClusterID, &role); err != nil {
			return nil, err
		}
		p.Role = Role(role)
		perms = append(perms, p)
	}
	return perms, nil
}

func (s *Store) SetUserClusterPermissions(userID int64, perms []UserClusterPermission) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM user_cluster_permissions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, p := range perms {
		if p.ClusterID == "" || !p.Role.Valid() {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO user_cluster_permissions(user_id, cluster_id, role) VALUES (?, ?, ?)`, userID, p.ClusterID, string(p.Role)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) EffectiveClusterRole(u *User, clusterID string) (Role, error) {
	if u == nil {
		return "", nil
	}
	if u.Role == RoleAdmin {
		return RoleAdmin, nil
	}
	// Check explicit per-cluster grant first.
	var r string
	err := s.db.QueryRow(`SELECT role FROM user_cluster_permissions WHERE user_id = ? AND cluster_id = ?`, u.ID, clusterID).Scan(&r)
	if err == nil {
		return Role(r), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	// If user has ANY explicit cluster permissions configured, deny unlisted clusters.
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM user_cluster_permissions WHERE user_id = ?`, u.ID).Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		return "", nil
	}
	// Otherwise fall back to global role.
	return u.Role, nil
}

// --- Clusters Metadata ---

func (s *Store) UpsertClusterMeta(m *ClusterMeta) error {
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	_, err := s.db.Exec(
		`INSERT INTO clusters_meta(cluster_id, display_name, description, caves_enabled, mod_sync_enabled, auto_restart, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(cluster_id) DO UPDATE SET
		   display_name = excluded.display_name,
		   description = excluded.description,
		   caves_enabled = excluded.caves_enabled,
		   mod_sync_enabled = excluded.mod_sync_enabled,
		   auto_restart = excluded.auto_restart,
		   updated_at = excluded.updated_at`,
		m.ClusterID, m.DisplayName, m.Description, boolInt(m.CavesEnabled), boolInt(m.ModSyncEnabled), boolInt(m.AutoRestart), m.CreatedAt, m.UpdatedAt,
	)
	return err
}

func (s *Store) GetClusterMeta(clusterID string) (*ClusterMeta, error) {
	m := &ClusterMeta{}
	var caves, modSync, autoRestart int
	err := s.db.QueryRow(
		`SELECT cluster_id, display_name, description, caves_enabled, mod_sync_enabled, auto_restart, created_at, updated_at FROM clusters_meta WHERE cluster_id = ?`,
		clusterID,
	).Scan(&m.ClusterID, &m.DisplayName, &m.Description, &caves, &modSync, &autoRestart, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.CavesEnabled = caves != 0
	m.ModSyncEnabled = modSync != 0
	m.AutoRestart = autoRestart != 0
	return m, nil
}

func (s *Store) ListClustersMeta() (map[string]ClusterMeta, error) {
	rows, err := s.db.Query(`SELECT cluster_id, display_name, description, caves_enabled, mod_sync_enabled, auto_restart, created_at, updated_at FROM clusters_meta`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]ClusterMeta)
	for rows.Next() {
		var m ClusterMeta
		var caves, modSync, autoRestart int
		if err := rows.Scan(&m.ClusterID, &m.DisplayName, &m.Description, &caves, &modSync, &autoRestart, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.CavesEnabled = caves != 0
		m.ModSyncEnabled = modSync != 0
		m.AutoRestart = autoRestart != 0
		res[m.ClusterID] = m
	}
	return res, nil
}

func (s *Store) DeleteClusterMeta(clusterID string) error {
	_, _ = s.db.Exec(`DELETE FROM user_cluster_permissions WHERE cluster_id = ?`, clusterID)
	_, err := s.db.Exec(`DELETE FROM clusters_meta WHERE cluster_id = ?`, clusterID)
	return err
}

// --- Backups ---

func (s *Store) CreateBackupRecord(b *BackupRecord) (*BackupRecord, error) {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
	res, err := s.db.Exec(
		`INSERT INTO backups(cluster_id, filename, size_bytes, trigger_type, note, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.ClusterID, b.Filename, b.SizeBytes, b.Trigger, b.Note, b.CreatedBy, b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	b.ID, _ = res.LastInsertId()
	return b, nil
}

func (s *Store) ListBackups(clusterID string) ([]BackupRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, cluster_id, filename, size_bytes, trigger_type, note, created_by, created_at FROM backups WHERE cluster_id = ? ORDER BY created_at DESC, id DESC`,
		clusterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []BackupRecord
	for rows.Next() {
		var b BackupRecord
		if err := rows.Scan(&b.ID, &b.ClusterID, &b.Filename, &b.SizeBytes, &b.Trigger, &b.Note, &b.CreatedBy, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, nil
}

func (s *Store) GetBackup(id int64) (*BackupRecord, error) {
	b := &BackupRecord{}
	err := s.db.QueryRow(
		`SELECT id, cluster_id, filename, size_bytes, trigger_type, note, created_by, created_at FROM backups WHERE id = ?`,
		id,
	).Scan(&b.ID, &b.ClusterID, &b.Filename, &b.SizeBytes, &b.Trigger, &b.Note, &b.CreatedBy, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (s *Store) DeleteBackupRecord(id int64) error {
	_, err := s.db.Exec(`DELETE FROM backups WHERE id = ?`, id)
	return err
}

// --- Audit Logs ---

func (s *Store) AddAuditLog(actor, action, target, detail, ip string) {
	_, _ = s.db.Exec(
		`INSERT INTO audit_logs(actor, action, target, detail, ip, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		actor, action, target, detail, ip, time.Now().UTC(),
	)
}

func (s *Store) ListAuditLogs(limit int, actionFilter string) ([]AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if strings.TrimSpace(actionFilter) != "" {
		rows, err = s.db.Query(
			`SELECT id, actor, action, target, detail, ip, created_at FROM audit_logs WHERE action LIKE ? ORDER BY id DESC LIMIT ?`,
			"%"+strings.TrimSpace(actionFilter)+"%", limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, actor, action, target, detail, ip, created_at FROM audit_logs ORDER BY id DESC LIMIT ?`,
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AuditLog
	for rows.Next() {
		var a AuditLog
		if err := rows.Scan(&a.ID, &a.Actor, &a.Action, &a.Target, &a.Detail, &a.IP, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --- Mod Cache ---

func (s *Store) UpsertModCache(m *ModCache) error {
	if m.LastSyncedAt.IsZero() {
		m.LastSyncedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO mod_cache(mod_id, workshop_id, is_local, name, author, version, description, preview_url, client_only, server_tags, time_updated, last_synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(mod_id) DO UPDATE SET
		   workshop_id = CASE WHEN excluded.workshop_id != '' THEN excluded.workshop_id ELSE mod_cache.workshop_id END,
		   is_local = excluded.is_local,
		   name = CASE WHEN excluded.name != '' THEN excluded.name ELSE mod_cache.name END,
		   author = CASE WHEN excluded.author != '' THEN excluded.author ELSE mod_cache.author END,
		   version = CASE WHEN excluded.version != '' THEN excluded.version ELSE mod_cache.version END,
		   description = CASE WHEN excluded.description != '' THEN excluded.description ELSE mod_cache.description END,
		   preview_url = CASE WHEN excluded.preview_url != '' THEN excluded.preview_url ELSE mod_cache.preview_url END,
		   client_only = excluded.client_only,
		   server_tags = CASE WHEN excluded.server_tags != '' THEN excluded.server_tags ELSE mod_cache.server_tags END,
		   time_updated = CASE WHEN excluded.time_updated > 0 THEN excluded.time_updated ELSE mod_cache.time_updated END,
		   last_synced_at = excluded.last_synced_at`,
		m.ModID, m.WorkshopID, boolInt(m.IsLocal), m.Name, m.Author, m.Version, m.Description, m.PreviewURL, boolInt(m.ClientOnly), m.ServerTags, m.TimeUpdated, m.LastSyncedAt,
	)
	return err
}

func (s *Store) GetModCache(modID string) (*ModCache, error) {
	m := &ModCache{}
	var isLocal, clientOnly int
	err := s.db.QueryRow(
		`SELECT mod_id, workshop_id, is_local, name, author, version, description, preview_url, client_only, server_tags, time_updated, last_synced_at FROM mod_cache WHERE mod_id = ?`,
		modID,
	).Scan(&m.ModID, &m.WorkshopID, &isLocal, &m.Name, &m.Author, &m.Version, &m.Description, &m.PreviewURL, &clientOnly, &m.ServerTags, &m.TimeUpdated, &m.LastSyncedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.IsLocal = isLocal != 0
	m.ClientOnly = clientOnly != 0
	return m, nil
}

func (s *Store) ListModCache() ([]ModCache, error) {
	rows, err := s.db.Query(`SELECT mod_id, workshop_id, is_local, name, author, version, description, preview_url, client_only, server_tags, time_updated, last_synced_at FROM mod_cache ORDER BY is_local DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []ModCache
	for rows.Next() {
		var m ModCache
		var isLocal, clientOnly int
		if err := rows.Scan(&m.ModID, &m.WorkshopID, &isLocal, &m.Name, &m.Author, &m.Version, &m.Description, &m.PreviewURL, &clientOnly, &m.ServerTags, &m.TimeUpdated, &m.LastSyncedAt); err != nil {
			return nil, err
		}
		m.IsLocal = isLocal != 0
		m.ClientOnly = clientOnly != 0
		list = append(list, m)
	}
	return list, nil
}

func (s *Store) DeleteModCache(modID string) error {
	_, err := s.db.Exec(`DELETE FROM mod_cache WHERE mod_id = ?`, modID)
	return err
}

// --- Player History ---

func (s *Store) UpsertPlayerHistory(kuID, name, clusterID string) error {
	kuID = strings.TrimSpace(kuID)
	if kuID == "" {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO player_history(ku_id, last_name, last_cluster_id, last_seen_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(ku_id) DO UPDATE SET
		   last_name = CASE WHEN excluded.last_name != '' THEN excluded.last_name ELSE player_history.last_name END,
		   last_cluster_id = CASE WHEN excluded.last_cluster_id != '' THEN excluded.last_cluster_id ELSE player_history.last_cluster_id END,
		   last_seen_at = excluded.last_seen_at`,
		kuID, strings.TrimSpace(name), clusterID, time.Now().UTC(),
	)
	return err
}

func (s *Store) ListPlayerHistory(limit int) ([]PlayerHistory, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT ku_id, last_name, last_cluster_id, last_seen_at FROM player_history ORDER BY last_seen_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []PlayerHistory
	for rows.Next() {
		var p PlayerHistory
		if err := rows.Scan(&p.KUID, &p.LastName, &p.LastClusterID, &p.LastSeenAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}

// --- Command Snippets ---

func (s *Store) ListCommandSnippets() ([]CommandSnippet, error) {
	rows, err := s.db.Query(`SELECT id, name, shard, command, category, created_at FROM command_snippets ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []CommandSnippet
	for rows.Next() {
		var c CommandSnippet
		if err := rows.Scan(&c.ID, &c.Name, &c.Shard, &c.Command, &c.Category, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (s *Store) CreateCommandSnippet(name, shard, command, category string) (*CommandSnippet, error) {
	if shard == "" {
		shard = "master"
	}
	if category == "" {
		category = "custom"
	}
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO command_snippets(name, shard, command, category, created_at) VALUES (?, ?, ?, ?, ?)`,
		strings.TrimSpace(name), shard, strings.TrimSpace(command), category, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &CommandSnippet{ID: id, Name: name, Shard: shard, Command: command, Category: category, CreatedAt: now}, nil
}

func (s *Store) DeleteCommandSnippet(id int64) error {
	_, err := s.db.Exec(`DELETE FROM command_snippets WHERE id = ?`, id)
	return err
}

// --- Scheduled Tasks ---

func (s *Store) ListScheduledTasks(clusterID string) ([]ScheduledTask, error) {
	var rows *sql.Rows
	var err error
	if strings.TrimSpace(clusterID) != "" {
		rows, err = s.db.Query(
			`SELECT id, cluster_id, name, task_type, cron_expr, params_json, enabled, last_run_at, last_status, created_at
			 FROM scheduled_tasks WHERE cluster_id = ? OR cluster_id = '*' ORDER BY id DESC`,
			clusterID,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, cluster_id, name, task_type, cron_expr, params_json, enabled, last_run_at, last_status, created_at
			 FROM scheduled_tasks ORDER BY id DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ScheduledTask
	for rows.Next() {
		var t ScheduledTask
		var enabled int
		var lastRun sql.NullTime
		if err := rows.Scan(&t.ID, &t.ClusterID, &t.Name, &t.TaskType, &t.CronExpr, &t.ParamsJSON, &enabled, &lastRun, &t.LastStatus, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Enabled = enabled != 0
		if lastRun.Valid {
			ts := lastRun.Time
			t.LastRunAt = &ts
		}
		list = append(list, t)
	}
	if list == nil {
		return []ScheduledTask{}, nil
	}
	return list, nil
}

func (s *Store) GetScheduledTask(id int64) (*ScheduledTask, error) {
	var t ScheduledTask
	var enabled int
	var lastRun sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, cluster_id, name, task_type, cron_expr, params_json, enabled, last_run_at, last_status, created_at FROM scheduled_tasks WHERE id = ?`,
		id,
	).Scan(&t.ID, &t.ClusterID, &t.Name, &t.TaskType, &t.CronExpr, &t.ParamsJSON, &enabled, &lastRun, &t.LastStatus, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Enabled = enabled != 0
	if lastRun.Valid {
		ts := lastRun.Time
		t.LastRunAt = &ts
	}
	return &t, nil
}

func (s *Store) CreateScheduledTask(t *ScheduledTask) (*ScheduledTask, error) {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(t.ParamsJSON) == "" {
		t.ParamsJSON = "{}"
	}
	res, err := s.db.Exec(
		`INSERT INTO scheduled_tasks(cluster_id, name, task_type, cron_expr, params_json, enabled, last_status, created_at) VALUES (?, ?, ?, ?, ?, ?, '', ?)`,
		t.ClusterID, strings.TrimSpace(t.Name), t.TaskType, strings.TrimSpace(t.CronExpr), t.ParamsJSON, boolInt(t.Enabled), t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	t.ID, _ = res.LastInsertId()
	return s.GetScheduledTask(t.ID)
}

func (s *Store) UpdateScheduledTask(t *ScheduledTask) (*ScheduledTask, error) {
	if strings.TrimSpace(t.ParamsJSON) == "" {
		t.ParamsJSON = "{}"
	}
	_, err := s.db.Exec(
		`UPDATE scheduled_tasks SET cluster_id = ?, name = ?, task_type = ?, cron_expr = ?, params_json = ?, enabled = ? WHERE id = ?`,
		t.ClusterID, strings.TrimSpace(t.Name), t.TaskType, strings.TrimSpace(t.CronExpr), t.ParamsJSON, boolInt(t.Enabled), t.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetScheduledTask(t.ID)
}

func (s *Store) RecordScheduledTaskRun(id int64, status string) {
	_, _ = s.db.Exec(`UPDATE scheduled_tasks SET last_run_at = ?, last_status = ? WHERE id = ?`, time.Now().UTC(), status, id)
}

func (s *Store) DeleteScheduledTask(id int64) error {
	_, err := s.db.Exec(`DELETE FROM scheduled_tasks WHERE id = ?`, id)
	return err
}

// --- Webhooks & Deliveries ---

func (s *Store) ListWebhooks() ([]WebhookConfig, error) {
	rows, err := s.db.Query(`SELECT id, name, template, url, secret, events_json, enabled, created_at FROM webhooks ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []WebhookConfig
	for rows.Next() {
		var w WebhookConfig
		var eventsJSON string
		var enabled int
		if err := rows.Scan(&w.ID, &w.Name, &w.Template, &w.URL, &w.Secret, &eventsJSON, &enabled, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Enabled = enabled != 0
		_ = json.Unmarshal([]byte(eventsJSON), &w.Events)
		if w.Events == nil {
			w.Events = []string{}
		}
		list = append(list, w)
	}
	if list == nil {
		return []WebhookConfig{}, nil
	}
	return list, nil
}

func (s *Store) GetWebhook(id int64) (*WebhookConfig, error) {
	var w WebhookConfig
	var eventsJSON string
	var enabled int
	err := s.db.QueryRow(`SELECT id, name, template, url, secret, events_json, enabled, created_at FROM webhooks WHERE id = ?`, id).
		Scan(&w.ID, &w.Name, &w.Template, &w.URL, &w.Secret, &eventsJSON, &enabled, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	w.Enabled = enabled != 0
	_ = json.Unmarshal([]byte(eventsJSON), &w.Events)
	if w.Events == nil {
		w.Events = []string{}
	}
	return &w, nil
}

func (s *Store) CreateWebhook(w *WebhookConfig) (*WebhookConfig, error) {
	evBytes, _ := json.Marshal(w.Events)
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO webhooks(name, template, url, secret, events_json, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(w.Name), w.Template, strings.TrimSpace(w.URL), strings.TrimSpace(w.Secret), string(evBytes), boolInt(w.Enabled), now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetWebhook(id)
}

func (s *Store) UpdateWebhook(w *WebhookConfig) (*WebhookConfig, error) {
	evBytes, _ := json.Marshal(w.Events)
	_, err := s.db.Exec(
		`UPDATE webhooks SET name = ?, template = ?, url = ?, secret = ?, events_json = ?, enabled = ? WHERE id = ?`,
		strings.TrimSpace(w.Name), w.Template, strings.TrimSpace(w.URL), strings.TrimSpace(w.Secret), string(evBytes), boolInt(w.Enabled), w.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetWebhook(w.ID)
}

func (s *Store) DeleteWebhook(id int64) error {
	_, err := s.db.Exec(`DELETE FROM webhooks WHERE id = ?`, id)
	return err
}

func (s *Store) AddWebhookDelivery(d *WebhookDelivery) {
	_, _ = s.db.Exec(
		`INSERT INTO webhook_deliveries(webhook_id, event, success, status_code, error, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		d.WebhookID, d.Event, boolInt(d.Success), d.StatusCode, d.Error, time.Now().UTC(),
	)
	// Keep latest 300 delivery rows to avoid unbounded table growth.
	_, _ = s.db.Exec(`DELETE FROM webhook_deliveries WHERE id NOT IN (SELECT id FROM webhook_deliveries ORDER BY id DESC LIMIT 300)`)
}

func (s *Store) ListWebhookDeliveries(limit int) ([]WebhookDelivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, webhook_id, event, success, status_code, error, created_at FROM webhook_deliveries ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		var success int
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Event, &success, &d.StatusCode, &d.Error, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.Success = success != 0
		list = append(list, d)
	}
	if list == nil {
		return []WebhookDelivery{}, nil
	}
	return list, nil
}
