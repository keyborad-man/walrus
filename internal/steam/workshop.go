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

type WorkshopClient struct {
	httpClient  *http.Client
	runtimeMode string
}

func NewWorkshopClient(runtimeMode string) *WorkshopClient {
	return &WorkshopClient{
		httpClient:  &http.Client{Timeout: 12 * time.Second},
		runtimeMode: runtimeMode,
	}
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

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.steampowered.com/ISteamRemoteStorage/GetPublishedFileDetails/v1/",
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

// SearchWorkshop searches DST Workshop items via Steam Web API (requires Steam Web API Key, or returns curated samples in mock mode).
func (c *WorkshopClient) SearchWorkshop(ctx context.Context, apiKey, query string, page int) ([]WorkshopItem, int, bool, error) {
	if page <= 0 {
		page = 1
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		if c.runtimeMode == "mock" {
			items := filterMockCatalog(query)
			return items, len(items), true, nil
		}
		return nil, 0, false, apperr.New(apperr.CodeSteamAPIKeyRequired, "Steam Web API Key is required in System Settings for Workshop keyword search (you can still add mods directly by Workshop ID, URL, or Collection without a key)")
	}

	params := url.Values{}
	params.Set("key", apiKey)
	params.Set("appid", DSTGameAppID)
	params.Set("search_text", strings.TrimSpace(query))
	params.Set("page", strconv.Itoa(page))
	params.Set("numperpage", "12")
	params.Set("return_short_description", "1")
	params.Set("return_tags", "1")
	params.Set("return_metadata", "1")
	params.Set("query_type", "1") // RankedByPublicationDate / Relevance when search_text set

	endpoint := "https://api.steampowered.com/IPublishedFileService/QueryFiles/v1/?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, false, apperr.WrapDetail(apperr.CodeWorkshopSearchFailed, err, "steam workshop search failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, 0, false, apperr.New(apperr.CodeSteamAPIKeyRejected, "Steam Web API Key rejected by Steam (HTTP 401/403)")
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

var mockPopularMods = []WorkshopItem{
	{
		WorkshopID: "378160973", ModID: "workshop-378160973",
		Title: "Global Positions (全球定位)", Description: "Share map positions, pings, and campfire beacons with teammates.",
		Subscriptions: 4820000, Tags: []string{"all_clients_require_mod", "map"},
	},
	{
		WorkshopID: "1207269058", ModID: "workshop-1207269058",
		Title: "Simple Health Bar DST (简易血条)", Description: "Displays clean health bars and floating damage numbers.",
		Subscriptions: 3910000, Tags: []string{"all_clients_require_mod", "ui"},
	},
	{
		WorkshopID: "375850599", ModID: "workshop-375850599",
		Title: "Extra Equip Slots (五格装备栏)", Description: "Adds dedicated backpack and amulet equipment slots.",
		Subscriptions: 3250000, Tags: []string{"all_clients_require_mod", "gameplay"},
	},
	{
		WorkshopID: "666155465", ModID: "workshop-666155465",
		Title: "Show Me (Origin) (信息显示)", Description: "Inspect food stats, container contents, and creature timers.",
		Subscriptions: 4120000, Tags: []string{"all_clients_require_mod", "info"},
	},
	{
		WorkshopID: "1185229307", ModID: "workshop-1185229307",
		Title: "Epic Healthbar (史诗级Boss血条)", Description: "Classic RPG boss health bar at the top of the screen.",
		Subscriptions: 2740000, Tags: []string{"all_clients_require_mod", "boss"},
	},
	{
		WorkshopID: "362175979", ModID: "workshop-362175979",
		Title: "Wormhole Marks (虫洞标记)", Description: "Color-codes paired wormholes on the map automatically.",
		Subscriptions: 2680000, Tags: []string{"all_clients_require_mod", "map"},
	},
}

func filterMockCatalog(q string) []WorkshopItem {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return mockPopularMods
	}
	var out []WorkshopItem
	for _, it := range mockPopularMods {
		if strings.Contains(strings.ToLower(it.Title), q) ||
			strings.Contains(strings.ToLower(it.Description), q) ||
			strings.Contains(it.WorkshopID, q) {
			out = append(out, it)
		}
	}
	return out
}

func mockFallbackDetails(ids []string) []WorkshopItem {
	byID := make(map[string]WorkshopItem, len(mockPopularMods))
	for _, m := range mockPopularMods {
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
