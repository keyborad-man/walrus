package steam

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"walrus/internal/apperr"
)

type WorkshopItem struct {
	WorkshopID    string   `json:"workshop_id"`
	ModID         string   `json:"mod_id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	PreviewURL    string   `json:"preview_url"`
	TimeUpdated   int64    `json:"time_updated"`
	Subscriptions int64    `json:"subscriptions"`
	FileSize      int64    `json:"file_size"`
	Tags          []string `json:"tags"`
	IsClientOnly  bool     `json:"is_client_only"`
}

const (
	defaultPublishedFileDetailsURL = "https://api.steampowered.com/ISteamRemoteStorage/GetPublishedFileDetails/v1/"
	defaultQueryFilesURL           = "https://api.steampowered.com/IPublishedFileService/QueryFiles/v1/"
	curatedCacheTTL                = 6 * time.Hour
	curatedRetryCooldown           = 2 * time.Minute
)

type WorkshopClient struct {
	httpClient        *http.Client
	runtimeMode       string
	defaultAPIKey     string
	detailsEndpoint   string
	queryEndpoint     string
	mu                sync.Mutex
	curatedCache      []WorkshopItem
	curatedCachedAt   time.Time
	curatedRetryAfter time.Time
}

func NewWorkshopClient(runtimeMode string) *WorkshopClient {
	return &WorkshopClient{
		httpClient:      &http.Client{Timeout: 12 * time.Second},
		runtimeMode:     runtimeMode,
		detailsEndpoint: defaultPublishedFileDetailsURL,
		queryEndpoint:   defaultQueryFilesURL,
	}
}

// SetDefaultAPIKey configures a fallback Steam Web API Key used when the user has not set a custom key in System Settings.
func (c *WorkshopClient) SetDefaultAPIKey(key string) {
	c.mu.Lock()
	c.defaultAPIKey = strings.TrimSpace(key)
	c.mu.Unlock()
}

func (c *WorkshopClient) getDefaultAPIKey() string {
	c.mu.Lock()
	k := c.defaultAPIKey
	c.mu.Unlock()
	return k
}

var (
	workshopURLIDRe = regexp.MustCompile(`[?&]id=(\d+)`)
	workshopPrefRe  = regexp.MustCompile(`^workshop-(\d+)$`)
	pureDigitsRe    = regexp.MustCompile(`^\d{4,15}$`)
)

// ExtractWorkshopID extracts the numeric Workshop ID from an ID, "workshop-123", or full Steam URL.
func ExtractWorkshopID(input string) (string, error) {
	s := strings.TrimSpace(input)
	if pureDigitsRe.MatchString(s) {
		return s, nil
	}
	if m := workshopPrefRe.FindStringSubmatch(s); len(m) == 2 {
		return m[1], nil
	}
	if m := workshopURLIDRe.FindStringSubmatch(s); len(m) == 2 {
		return m[1], nil
	}
	return "", apperr.Newf(apperr.CodeInvalidWorkshopID, map[string]any{"input": input}, "could not extract a valid Steam Workshop ID from %q", input)
}

// GetPublishedFileDetails fetches metadata for one or more Workshop IDs without requiring an API key.
func (c *WorkshopClient) GetPublishedFileDetails(ctx context.Context, ids []string) ([]WorkshopItem, error) {
	if len(ids) == 0 {
		return []WorkshopItem{}, nil
	}
	form := url.Values{}
	form.Set("itemcount", strconv.Itoa(len(ids)))
	for i, id := range ids {
		form.Set(fmt.Sprintf("publishedfileids[%d]", i), id)
	}

	detailsURL := c.detailsEndpoint
	if detailsURL == "" {
		detailsURL = defaultPublishedFileDetailsURL
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		detailsURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.runtimeMode == "mock" {
			return mockFallbackDetails(ids), nil
		}
		return nil, apperr.WrapDetail(apperr.CodeWorkshopAPIFailed, err, "steam workshop API request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Response struct {
			PublishedFileDetails []struct {
				PublishedFileID string `json:"publishedfileid"`
				Result          int    `json:"result"`
				Title           string `json:"title"`
				Description     string `json:"description"`
				PreviewURL      string `json:"preview_url"`
				TimeUpdated     int64  `json:"time_updated"`
				Subscriptions   int64  `json:"subscriptions"`
				FileSize        any    `json:"file_size"`
				Tags            []struct {
					Tag string `json:"tag"`
				} `json:"tags"`
			} `json:"publishedfiledetails"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, apperr.WrapDetail(apperr.CodeWorkshopAPIFailed, err, "decode workshop details: %v", err)
	}

	out := make([]WorkshopItem, 0, len(payload.Response.PublishedFileDetails))
	for _, d := range payload.Response.PublishedFileDetails {
		if d.Result != 1 || d.PublishedFileID == "" {
			continue
		}
		tags := make([]string, 0, len(d.Tags))
		clientOnly := false
		for _, tg := range d.Tags {
			tags = append(tags, tg.Tag)
			if strings.EqualFold(tg.Tag, "client_only_mod") {
				clientOnly = true
			}
		}
		out = append(out, WorkshopItem{
			WorkshopID:    d.PublishedFileID,
			ModID:         "workshop-" + d.PublishedFileID,
			Title:         d.Title,
			Description:   d.Description,
			PreviewURL:    d.PreviewURL,
			TimeUpdated:   d.TimeUpdated,
			Subscriptions: d.Subscriptions,
			FileSize:      parseAnyInt64(d.FileSize),
			Tags:          tags,
			IsClientOnly:  clientOnly,
		})
	}
	return out, nil
}

// GetCollectionItems resolves a Steam Workshop Collection ID/URL to all child Workshop IDs.
func (c *WorkshopClient) GetCollectionItems(ctx context.Context, input string) ([]string, error) {
	colID, err := ExtractWorkshopID(input)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("collectioncount", "1")
	form.Set("publishedfileids[0]", colID)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.steampowered.com/ISteamRemoteStorage/GetCollectionDetails/v1/",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.runtimeMode == "mock" {
			return []string{"378160973", "1207269058", "375850599"}, nil
		}
		return nil, apperr.WrapDetail(apperr.CodeWorkshopCollectionAPIFail, err, "steam collection API failed: %v", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Response struct {
			CollectionDetails []struct {
				Result   int `json:"result"`
				Children []struct {
					PublishedFileID string `json:"publishedfileid"`
				} `json:"children"`
			} `json:"collectiondetails"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, apperr.WrapDetail(apperr.CodeWorkshopCollectionAPIFail, err, "decode collection details: %v", err)
	}
	if len(payload.Response.CollectionDetails) == 0 || payload.Response.CollectionDetails[0].Result != 1 {
		return nil, apperr.New(apperr.CodeWorkshopCollectionEmpty, "workshop collection not found or empty")
	}

	var ids []string
	for _, ch := range payload.Response.CollectionDetails[0].Children {
		if ch.PublishedFileID != "" {
			ids = append(ids, ch.PublishedFileID)
		}
	}
	return ids, nil
}

// SearchWorkshop searches DST Workshop items via Steam Web API.
// Precedence: user-configured apiKey > defaultAPIKey > curated popular catalog.
// Direct Workshop ID / Steam URL queries always resolve via keyless GetPublishedFileDetails.
// If a defaultAPIKey request fails or is rejected, it degrades gracefully to the curated catalog.
func (c *WorkshopClient) SearchWorkshop(ctx context.Context, apiKey, query string, page int) ([]WorkshopItem, int, bool, error) {
	if page <= 0 {
		page = 1
	}
	userKey := strings.TrimSpace(apiKey)
	effectiveKey := userKey
	usingDefaultKey := false
	if effectiveKey == "" {
		if defKey := c.getDefaultAPIKey(); defKey != "" {
			effectiveKey = defKey
			usingDefaultKey = true
		}
	}

	trimmed := strings.TrimSpace(query)
	if trimmed != "" {
		if wid, err := ExtractWorkshopID(trimmed); err == nil {
			items, err := c.GetPublishedFileDetails(ctx, []string{wid})
			if err != nil {
				return nil, 0, effectiveKey == "", err
			}
			return items, len(items), effectiveKey == "", nil
		}
	}

	if effectiveKey == "" {
		catalog := c.getCuratedCatalog(ctx)
		items := filterCuratedCatalog(catalog, trimmed)
		return items, len(items), true, nil
	}

	params := url.Values{}
	params.Set("key", effectiveKey)
	params.Set("appid", DSTGameAppID)
	params.Set("search_text", trimmed)
	params.Set("page", strconv.Itoa(page))
	params.Set("numperpage", "12")
	params.Set("return_short_description", "1")
	params.Set("return_tags", "1")
	params.Set("return_metadata", "1")
	if trimmed == "" {
		params.Set("query_type", "9") // RankedByTotalUniqueSubscriptions (popular mods)
	} else {
		params.Set("query_type", "12") // RankedByTextSearch
	}

	queryBase := c.queryEndpoint
	if queryBase == "" {
		queryBase = defaultQueryFilesURL
	}
	endpoint := queryBase + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if usingDefaultKey {
			catalog := c.getCuratedCatalog(ctx)
			items := filterCuratedCatalog(catalog, trimmed)
			return items, len(items), true, nil
		}
		return nil, 0, false, apperr.WrapDetail(apperr.CodeWorkshopSearchFailed, err, "steam workshop search failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		if usingDefaultKey {
			catalog := c.getCuratedCatalog(ctx)
			items := filterCuratedCatalog(catalog, trimmed)
			return items, len(items), true, nil
		}
		return nil, 0, false, apperr.New(apperr.CodeSteamAPIKeyRejected, "Steam Web API Key rejected by Steam (HTTP 401/403)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if usingDefaultKey {
			catalog := c.getCuratedCatalog(ctx)
			items := filterCuratedCatalog(catalog, trimmed)
			return items, len(items), true, nil
		}
		return nil, 0, false, apperr.Newf(apperr.CodeWorkshopSearchFailed, nil, "steam workshop search returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Response struct {
			Total                int `json:"total"`
			PublishedFileDetails []struct {
				PublishedFileID  string `json:"publishedfileid"`
				Title            string `json:"title"`
				ShortDescription string `json:"short_description"`
				PreviewURL       string `json:"preview_url"`
				TimeUpdated      int64  `json:"time_updated"`
				Subscriptions    int64  `json:"subscriptions"`
				FileSize         any    `json:"file_size"`
				Tags             []struct {
					Tag string `json:"tag"`
				} `json:"tags"`
			} `json:"publishedfiledetails"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, false, err
	}

	items := make([]WorkshopItem, 0, len(payload.Response.PublishedFileDetails))
	for _, d := range payload.Response.PublishedFileDetails {
		if d.PublishedFileID == "" {
			continue
		}
		tags := make([]string, 0, len(d.Tags))
		clientOnly := false
		for _, tg := range d.Tags {
			tags = append(tags, tg.Tag)
			if strings.EqualFold(tg.Tag, "client_only_mod") {
				clientOnly = true
			}
		}
		items = append(items, WorkshopItem{
			WorkshopID:    d.PublishedFileID,
			ModID:         "workshop-" + d.PublishedFileID,
			Title:         d.Title,
			Description:   d.ShortDescription,
			PreviewURL:    d.PreviewURL,
			TimeUpdated:   d.TimeUpdated,
			Subscriptions: d.Subscriptions,
			FileSize:      parseAnyInt64(d.FileSize),
			Tags:          tags,
			IsClientOnly:  clientOnly,
		})
	}
	return items, payload.Response.Total, false, nil
}

func parseAnyInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		return 0
	}
}

var curatedPopularMods = []WorkshopItem{
	{
		WorkshopID: "378160973", ModID: "workshop-378160973",
		Title: "Global Positions (全球定位)", Description: "Share map positions, pings, and campfire beacons with teammates.",
		Subscriptions: 8635000, Tags: []string{"all_clients_require_mod", "map"},
	},
	{
		WorkshopID: "1207269058", ModID: "workshop-1207269058",
		Title: "Simple Health Bar DST (简易血条)", Description: "Displays clean health bars and floating damage numbers.",
		Subscriptions: 5350000, Tags: []string{"all_clients_require_mod", "ui"},
	},
	{
		WorkshopID: "666155465", ModID: "workshop-666155465",
		Title: "Show Me (Origin) (信息显示)", Description: "Inspect food stats, container contents, and creature timers.",
		Subscriptions: 3192000, Tags: []string{"all_clients_require_mod", "info"},
	},
	{
		WorkshopID: "2189004162", ModID: "workshop-2189004162",
		Title: "Insight (Show Me+) (详细信息增强)", Description: "Modern Show Me successor showing detailed item, creature, and respawn info.",
		Subscriptions: 1530000, Tags: []string{"all_clients_require_mod", "info"},
	},
	{
		WorkshopID: "1185229307", ModID: "workshop-1185229307",
		Title: "Epic Healthbar (史诗级Boss血条)", Description: "Classic RPG boss health bar at the top of the screen.",
		Subscriptions: 3756000, Tags: []string{"all_clients_require_mod", "boss"},
	},
	{
		WorkshopID: "362175979", ModID: "workshop-362175979",
		Title: "Wormhole Marks (虫洞标记)", Description: "Color-codes paired wormholes on the map automatically.",
		Subscriptions: 3589000, Tags: []string{"all_clients_require_mod", "map"},
	},
	{
		WorkshopID: "374550642", ModID: "workshop-374550642",
		Title: "Increased Stack size (物品堆叠上限)", Description: "Increases maximum item stack sizes in inventory and chests (default 99).",
		Subscriptions: 2853000, Tags: []string{"all_clients_require_mod", "tweaks"},
	},
	{
		WorkshopID: "501385076", ModID: "workshop-501385076",
		Title: "Quick Pick (快速采集)", Description: "Speeds up harvesting grass, saplings, berry bushes, and more.",
		Subscriptions: 2544000, Tags: []string{"all_clients_require_mod", "tweaks"},
	},
	{
		WorkshopID: "1595631294", ModID: "workshop-1595631294",
		Title: "Smart Minisign (智能小木牌)", Description: "Automatically draws chest contents on nearby mini signs for easy storage.",
		Subscriptions: 1667000, Tags: []string{"all_clients_require_mod", "storage"},
	},
	{
		WorkshopID: "569043634", ModID: "workshop-569043634",
		Title: "Campfire Respawn (营火重生)", Description: "Allows ghost players to resurrect at lit campfires and endothermic fire pits.",
		Subscriptions: 1589000, Tags: []string{"all_clients_require_mod", "gameplay"},
	},
	{
		WorkshopID: "466732225", ModID: "workshop-466732225",
		Title: "No Thermal Stone Durability (暖石无限耐久)", Description: "Removes durability loss on thermal stones across seasons.",
		Subscriptions: 1562000, Tags: []string{"all_clients_require_mod", "tweaks"},
	},
	{
		WorkshopID: "1530801499", ModID: "workshop-1530801499",
		Title: "Fast Travel (GUI) (木牌传送)", Description: "Fast travel between discovered signpost locations via a clean UI.",
		Subscriptions: 1034000, Tags: []string{"all_clients_require_mod", "utility"},
	},
}

func cloneWorkshopItems(src []WorkshopItem) []WorkshopItem {
	out := make([]WorkshopItem, len(src))
	for i, it := range src {
		if len(it.Tags) > 0 {
			tags := make([]string, len(it.Tags))
			copy(tags, it.Tags)
			it.Tags = tags
		}
		out[i] = it
	}
	return out
}

func (c *WorkshopClient) getCuratedCatalog(ctx context.Context) []WorkshopItem {
	now := time.Now()
	c.mu.Lock()
	if len(c.curatedCache) > 0 && now.Sub(c.curatedCachedAt) < curatedCacheTTL {
		cached := cloneWorkshopItems(c.curatedCache)
		c.mu.Unlock()
		return cached
	}
	if c.runtimeMode == "mock" || (!c.curatedRetryAfter.IsZero() && now.Before(c.curatedRetryAfter)) {
		c.mu.Unlock()
		return cloneWorkshopItems(curatedPopularMods)
	}
	c.mu.Unlock()

	ids := make([]string, len(curatedPopularMods))
	for i, m := range curatedPopularMods {
		ids[i] = m.WorkshopID
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	details, err := c.GetPublishedFileDetails(reqCtx, ids)
	if err != nil || len(details) == 0 {
		c.mu.Lock()
		c.curatedRetryAfter = time.Now().Add(curatedRetryCooldown)
		c.mu.Unlock()
		return cloneWorkshopItems(curatedPopularMods)
	}

	byID := make(map[string]WorkshopItem, len(details))
	for _, d := range details {
		byID[d.WorkshopID] = d
	}

	merged := cloneWorkshopItems(curatedPopularMods)
	for i := range merged {
		if d, ok := byID[merged[i].WorkshopID]; ok {
			if d.PreviewURL != "" {
				merged[i].PreviewURL = d.PreviewURL
			}
			if d.TimeUpdated > 0 {
				merged[i].TimeUpdated = d.TimeUpdated
			}
			if d.Subscriptions > 0 {
				merged[i].Subscriptions = d.Subscriptions
			}
			if d.FileSize > 0 {
				merged[i].FileSize = d.FileSize
			}
			if len(d.Tags) > 0 {
				merged[i].Tags = d.Tags
			}
			merged[i].IsClientOnly = d.IsClientOnly
		}
	}

	c.mu.Lock()
	c.curatedCache = cloneWorkshopItems(merged)
	c.curatedCachedAt = time.Now()
	c.curatedRetryAfter = time.Time{}
	c.mu.Unlock()

	return merged
}

func filterCuratedCatalog(catalog []WorkshopItem, q string) []WorkshopItem {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return catalog
	}
	out := make([]WorkshopItem, 0)
	for _, it := range catalog {
		matched := strings.Contains(strings.ToLower(it.Title), q) ||
			strings.Contains(strings.ToLower(it.Description), q) ||
			strings.Contains(it.WorkshopID, q) ||
			strings.Contains(strings.ToLower(it.ModID), q)
		if !matched {
			for _, tag := range it.Tags {
				if strings.Contains(strings.ToLower(tag), q) {
					matched = true
					break
				}
			}
		}
		if matched {
			out = append(out, it)
		}
	}
	return out
}

func mockFallbackDetails(ids []string) []WorkshopItem {
	byID := make(map[string]WorkshopItem, len(curatedPopularMods))
	for _, m := range curatedPopularMods {
		byID[m.WorkshopID] = m
	}
	out := make([]WorkshopItem, 0, len(ids))
	for _, id := range ids {
		if m, ok := byID[id]; ok {
			out = append(out, m)
		} else {
			out = append(out, WorkshopItem{
				WorkshopID:  id,
				ModID:       "workshop-" + id,
				Title:       "Workshop Mod #" + id,
				Description: "DST Steam Workshop item #" + id,
			})
		}
	}
	return out
}
