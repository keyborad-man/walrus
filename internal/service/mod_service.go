package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"walrus/internal/apperr"
	"walrus/internal/db"
	"walrus/internal/dst"
	"walrus/internal/lua"
	"walrus/internal/steam"
)

type ClusterModItem struct {
	ModID                string                `json:"mod_id"`
	WorkshopID           string                `json:"workshop_id,omitempty"`
	IsLocal              bool                  `json:"is_local"`
	Enabled              bool                  `json:"enabled"`
	Downloaded           bool                  `json:"downloaded"`
	Name                 string                `json:"name"`
	Author               string                `json:"author"`
	Version              string                `json:"version"`
	Description          string                `json:"description"`
	PreviewURL           string                `json:"preview_url,omitempty"`
	ClientOnly           bool                  `json:"client_only"`
	AllClientsRequireMod bool                  `json:"all_clients_require_mod"`
	SchemaOptions        []lua.ModConfigOption `json:"schema_options"`
	ConfigurationOptions map[string]any        `json:"configuration_options"`
}

type ClusterModsResponse struct {
	Shard          string           `json:"shard"`
	ModSyncEnabled bool             `json:"mod_sync_enabled"`
	RawLua         string           `json:"raw_lua"`
	Mods           []ClusterModItem `json:"mods"`
}

const (
	previewFetchTimeout   = 5 * time.Second
	previewRetryCooldown  = 5 * time.Minute
	previewBatchChunkSize = 50
)

type ModService struct {
	dstServerDir      string
	localModsDir      string
	store             *db.Store
	fs                *dst.FS
	steamMgr          *steam.Manager
	workshop          *steam.WorkshopClient
	prepareSem        chan struct{}
	previewRetryMu    sync.Mutex
	previewRetryAfter map[string]time.Time
}

func NewModService(dstServerDir, localModsDir string, store *db.Store, fs *dst.FS, steamMgr *steam.Manager, workshop *steam.WorkshopClient) *ModService {
	return &ModService{
		dstServerDir:      dstServerDir,
		localModsDir:      localModsDir,
		store:             store,
		fs:                fs,
		steamMgr:          steamMgr,
		workshop:          workshop,
		prepareSem:        make(chan struct{}, 1),
		previewRetryAfter: make(map[string]time.Time),
	}
}

func (s *ModService) dstModsDir() string {
	return filepath.Join(s.dstServerDir, "mods")
}

func (s *ModService) hasInstalledDSTMod(modID string) bool {
	fi, err := os.Stat(filepath.Join(s.dstModsDir(), modID, "modinfo.lua"))
	return err == nil && !fi.IsDir()
}

// EnsureShardModsReady verifies that every enabled mod in a shard's modoverrides.lua
// is present under dst-server/mods/<modID>. Missing Workshop mods are downloaded
// automatically via SteamCMD in a single batch; missing local mods are mirrored from
// localModsDir when available.
func (s *ModService) EnsureShardModsReady(ctx context.Context, clusterID, shard string, logFn func(level, msg string)) error {
	emit := func(level, msg string) {
		if logFn != nil {
			logFn(level, msg)
		}
	}

	// Serialize concurrent shard starts (e.g., Master + Caves starting together)
	// so the second shard reuses mods already downloaded by the first instead of hitting STEAMCMD_BUSY.
	if s.prepareSem != nil {
		select {
		case s.prepareSem <- struct{}{}:
			defer func() { <-s.prepareSem }()
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	raw, err := s.fs.ReadModOverrides(clusterID, shard)
	if err != nil {
		return err
	}
	overrides, err := lua.ParseModOverrides(raw)
	if err != nil {
		return err
	}

	modIDs := make([]string, 0, len(overrides))
	for id, ov := range overrides {
		if ov.Enabled {
			modIDs = append(modIDs, id)
		}
	}
	sort.Strings(modIDs)

	var missingWorkshopIDs []string
	var missingLocalIDs []string
	for _, modID := range modIDs {
		if s.hasInstalledDSTMod(modID) {
			continue
		}
		if strings.HasPrefix(modID, "workshop-") {
			wid := strings.TrimSpace(strings.TrimPrefix(modID, "workshop-"))
			if wid != "" {
				missingWorkshopIDs = append(missingWorkshopIDs, wid)
			}
			continue
		}
		// Local mod: mirror from localModsDir into dst-server/mods if available.
		srcDir := filepath.Join(s.localModsDir, modID)
		if fi, statErr := os.Stat(filepath.Join(srcDir, "modinfo.lua")); statErr == nil && !fi.IsDir() {
			if copyErr := copyDirSimple(srcDir, filepath.Join(s.dstModsDir(), modID)); copyErr != nil {
				return fmt.Errorf("mirror local mod %s: %w", modID, copyErr)
			}
			emit("system", fmt.Sprintf("[walrus] Mirrored local mod %s into DST mods directory.", modID))
			continue
		}
		missingLocalIDs = append(missingLocalIDs, modID)
	}

	if len(missingLocalIDs) > 0 {
		return apperr.Newf(
			apperr.CodeLocalModNotFound,
			map[string]any{"mod_id": strings.Join(missingLocalIDs, ", ")},
			"enabled local mod(s) missing on disk: %s",
			strings.Join(missingLocalIDs, ", "),
		)
	}
	if len(missingWorkshopIDs) == 0 {
		return nil
	}

	emit("system", fmt.Sprintf("[walrus] %d enabled Workshop mod(s) missing locally; downloading via SteamCMD before launching %s...", len(missingWorkshopIDs), shard))
	if err := s.steamMgr.DownloadWorkshopMods(ctx, missingWorkshopIDs, func(msg string) {
		emit("system", msg)
	}); err != nil {
		return err
	}

	// Best-effort refresh cached metadata (including Steam preview icons) for newly downloaded mods.
	detailMap := s.fetchWorkshopDetails(ctx, missingWorkshopIDs)
	for _, wid := range missingWorkshopIDs {
		modID := "workshop-" + wid
		_, info, cache, _ := s.inspectMod(modID, "zh-CN")
		d, hasDetail := detailMap[wid]
		if info == nil && !hasDetail {
			continue
		}
		if cache == nil {
			cache = &db.ModCache{ModID: modID, WorkshopID: wid}
		}
		if hasDetail {
			mergeWorkshopDetailIntoCache(cache, d, info == nil)
		}
		if info != nil {
			if info.Name != "" {
				cache.Name = info.Name
			}
			cache.Author = info.Author
			cache.Version = info.Version
			if info.Description != "" && cache.Description == "" {
				cache.Description = info.Description
			}
			cache.ClientOnly = info.ClientOnlyMod
		}
		_ = s.store.UpsertModCache(cache)
	}
	_ = s.SyncDedicatedServerModsSetup()
	return nil
}

func (s *ModService) findModInfoPath(modID string) string {
	candidates := []string{
		filepath.Join(s.dstModsDir(), modID, "modinfo.lua"),
		filepath.Join(s.localModsDir, modID, "modinfo.lua"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (s *ModService) inspectMod(modID, locale string) (downloaded bool, info *lua.ModInfo, cache *db.ModCache, parseErr error) {
	cache, _ = s.store.GetModCache(modID)
	p := s.findModInfoPath(modID)
	if p == "" {
		return false, nil, cache, nil
	}
	downloaded = true
	raw, err := os.ReadFile(p)
	if err != nil {
		return downloaded, nil, cache, err
	}
	parsed, err := lua.ParseModInfo(string(raw), locale, modID)
	if err != nil {
		return downloaded, nil, cache, err
	}
	return downloaded, parsed, cache, nil
}

func (s *ModService) fetchWorkshopDetails(ctx context.Context, workshopIDs []string) map[string]steam.WorkshopItem {
	if s.workshop == nil || len(workshopIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(workshopIDs))
	unique := make([]string, 0, len(workshopIDs))
	for _, wid := range workshopIDs {
		wid = strings.TrimSpace(wid)
		if wid == "" {
			continue
		}
		if _, ok := seen[wid]; ok {
			continue
		}
		seen[wid] = struct{}{}
		unique = append(unique, wid)
	}
	if len(unique) == 0 {
		return nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, previewFetchTimeout)
	defer cancel()

	detailMap := make(map[string]steam.WorkshopItem, len(unique))
	for start := 0; start < len(unique); start += previewBatchChunkSize {
		end := start + previewBatchChunkSize
		if end > len(unique) {
			end = len(unique)
		}
		details, err := s.workshop.GetPublishedFileDetails(reqCtx, unique[start:end])
		if err != nil {
			continue
		}
		for _, d := range details {
			if d.WorkshopID != "" {
				detailMap[d.WorkshopID] = d
			}
		}
	}
	return detailMap
}

func mergeWorkshopDetailIntoCache(cache *db.ModCache, d steam.WorkshopItem, useSteamClientOnly bool) {
	if d.PreviewURL != "" {
		cache.PreviewURL = d.PreviewURL
	}
	if cache.Name == "" && d.Title != "" {
		cache.Name = d.Title
	}
	if cache.Description == "" && d.Description != "" {
		cache.Description = d.Description
	}
	if d.TimeUpdated > 0 {
		cache.TimeUpdated = d.TimeUpdated
	}
	if len(d.Tags) > 0 && cache.ServerTags == "" {
		cache.ServerTags = strings.Join(d.Tags, ",")
	}
	if useSteamClientOnly {
		cache.ClientOnly = d.IsClientOnly
	}
}

func (s *ModService) backfillClusterModPreviews(ctx context.Context, items []ClusterModItem) {
	if s.workshop == nil || len(items) == 0 {
		return
	}
	now := time.Now()
	var missingIdx []int
	var missingIDs []string

	s.previewRetryMu.Lock()
	for i := range items {
		it := &items[i]
		if it.IsLocal || it.WorkshopID == "" || it.PreviewURL != "" {
			continue
		}
		if retryAt, ok := s.previewRetryAfter[it.WorkshopID]; ok && now.Before(retryAt) {
			continue
		}
		missingIdx = append(missingIdx, i)
		missingIDs = append(missingIDs, it.WorkshopID)
	}
	s.previewRetryMu.Unlock()

	if len(missingIDs) == 0 {
		return
	}

	detailMap := s.fetchWorkshopDetails(ctx, missingIDs)
	retryUntil := time.Now().Add(previewRetryCooldown)

	s.previewRetryMu.Lock()
	defer s.previewRetryMu.Unlock()

	for _, idx := range missingIdx {
		it := &items[idx]
		d, ok := detailMap[it.WorkshopID]
		if !ok || d.PreviewURL == "" {
			s.previewRetryAfter[it.WorkshopID] = retryUntil
			continue
		}
		delete(s.previewRetryAfter, it.WorkshopID)

		it.PreviewURL = d.PreviewURL
		if it.Name == it.ModID && d.Title != "" {
			it.Name = d.Title
		}
		if it.Description == "" && d.Description != "" {
			it.Description = d.Description
		}

		cache, _ := s.store.GetModCache(it.ModID)
		if cache == nil {
			cache = &db.ModCache{
				ModID:       it.ModID,
				WorkshopID:  it.WorkshopID,
				Name:        it.Name,
				Author:      it.Author,
				Version:     it.Version,
				Description: it.Description,
				ClientOnly:  it.ClientOnly,
			}
		}
		mergeWorkshopDetailIntoCache(cache, d, !it.Downloaded)
		_ = s.store.UpsertModCache(cache)
	}
}

func (s *ModService) GetClusterMods(ctx context.Context, clusterID, shard, locale string) (*ClusterModsResponse, error) {
	if !s.fs.ClusterExists(clusterID) {
		return nil, db.ErrNotFound
	}
	shard = strings.ToLower(shard)
	if shard != "caves" {
		shard = "master"
	}

	rawLua, err := s.fs.ReadModOverrides(clusterID, shard)
	if err != nil {
		return nil, err
	}
	parsedOverrides, err := lua.ParseModOverrides(rawLua)
	if err != nil {
		parsedOverrides = map[string]lua.ModOverride{}
	}

	modSync := true
	if meta, err := s.store.GetClusterMeta(clusterID); err == nil {
		modSync = meta.ModSyncEnabled
	}

	modIDs := make([]string, 0, len(parsedOverrides))
	for id := range parsedOverrides {
		modIDs = append(modIDs, id)
	}
	sort.Strings(modIDs)

	items := make([]ClusterModItem, 0, len(modIDs))
	for _, modID := range modIDs {
		ov := parsedOverrides[modID]
		downloaded, info, cache, _ := s.inspectMod(modID, locale)

		item := ClusterModItem{
			ModID:                modID,
			IsLocal:              !strings.HasPrefix(modID, "workshop-"),
			Enabled:              ov.Enabled,
			Downloaded:           downloaded,
			Name:                 modID,
			SchemaOptions:        []lua.ModConfigOption{},
			ConfigurationOptions: ov.ConfigurationOptions,
		}
		if strings.HasPrefix(modID, "workshop-") {
			item.WorkshopID = strings.TrimPrefix(modID, "workshop-")
		}
		if item.ConfigurationOptions == nil {
			item.ConfigurationOptions = map[string]any{}
		}
		if cache != nil {
			if cache.Name != "" {
				item.Name = cache.Name
			}
			item.Author = cache.Author
			item.Version = cache.Version
			item.Description = cache.Description
			item.PreviewURL = cache.PreviewURL
			item.ClientOnly = cache.ClientOnly
			item.IsLocal = cache.IsLocal
		}
		if info != nil {
			if info.Name != "" {
				item.Name = info.Name
			}
			if info.Author != "" {
				item.Author = info.Author
			}
			if info.Version != "" {
				item.Version = info.Version
			}
			if info.Description != "" {
				item.Description = info.Description
			}
			item.ClientOnly = info.ClientOnlyMod
			item.AllClientsRequireMod = info.AllClientsRequireMod
			item.SchemaOptions = info.ConfigurationOptions
		}
		items = append(items, item)
	}

	s.backfillClusterModPreviews(ctx, items)

	return &ClusterModsResponse{
		Shard:          shard,
		ModSyncEnabled: modSync,
		RawLua:         rawLua,
		Mods:           items,
	}, nil
}

func (s *ModService) writeClusterOverrides(clusterID, shard string, overrides map[string]lua.ModOverride) error {
	content := lua.SerializeModOverrides(overrides)
	if err := s.fs.WriteModOverrides(clusterID, shard, content); err != nil {
		return err
	}

	// Sync to sibling shard if mod_sync_enabled and Caves exist.
	meta, err := s.store.GetClusterMeta(clusterID)
	if err == nil && meta.ModSyncEnabled && meta.CavesEnabled {
		sibling := "caves"
		if strings.ToLower(shard) == "caves" {
			sibling = "master"
		}
		_ = s.fs.WriteModOverrides(clusterID, sibling, content)
	}
	return s.SyncDedicatedServerModsSetup()
}

// AddWorkshopInput accepts a Workshop ID, Steam URL, or Collection ID/URL and adds all resolved mods to the cluster.
func (s *ModService) AddWorkshopInput(ctx context.Context, clusterID, shard, input string, isCollection bool, locale string) ([]string, error) {
	if !s.fs.ClusterExists(clusterID) {
		return nil, db.ErrNotFound
	}
	var ids []string
	if isCollection {
		colIDs, err := s.workshop.GetCollectionItems(ctx, input)
		if err != nil {
			return nil, err
		}
		ids = colIDs
	} else {
		id, err := steam.ExtractWorkshopID(input)
		if err != nil {
			return nil, err
		}
		ids = []string{id}
	}
	if len(ids) == 0 {
		return nil, apperr.New(apperr.CodeNoWorkshopIDs, "no workshop mod IDs found")
	}

	// Fetch Steam metadata (best effort).
	details, _ := s.workshop.GetPublishedFileDetails(ctx, ids)
	detailMap := make(map[string]steam.WorkshopItem, len(details))
	for _, d := range details {
		detailMap[d.WorkshopID] = d
	}

	raw, err := s.fs.ReadModOverrides(clusterID, shard)
	if err != nil {
		return nil, err
	}
	overrides, err := lua.ParseModOverrides(raw)
	if err != nil {
		overrides = map[string]lua.ModOverride{}
	}

	added := make([]string, 0, len(ids))
	for _, wid := range ids {
		modID := "workshop-" + wid
		// Download mod locally via SteamCMD so we can parse modinfo.lua immediately.
		_, _ = s.steamMgr.DownloadWorkshopMod(ctx, wid)

		// Cache combined metadata from Steam API + parsed modinfo.lua.
		cacheEntry := &db.ModCache{
			ModID:      modID,
			WorkshopID: wid,
			IsLocal:    false,
		}
		if d, ok := detailMap[wid]; ok {
			cacheEntry.Name = d.Title
			cacheEntry.Description = d.Description
			cacheEntry.PreviewURL = d.PreviewURL
			cacheEntry.TimeUpdated = d.TimeUpdated
			cacheEntry.ClientOnly = d.IsClientOnly
			cacheEntry.ServerTags = strings.Join(d.Tags, ",")
		}
		_, info, _, _ := s.inspectMod(modID, locale)
		defaults := map[string]any{}
		if info != nil {
			if info.Name != "" {
				cacheEntry.Name = info.Name
			}
			cacheEntry.Author = info.Author
			cacheEntry.Version = info.Version
			if info.Description != "" && cacheEntry.Description == "" {
				cacheEntry.Description = info.Description
			}
			cacheEntry.ClientOnly = info.ClientOnlyMod
			for _, opt := range info.ConfigurationOptions {
				if opt.Default != nil {
					defaults[opt.Name] = opt.Default
				}
			}
		}
		_ = s.store.UpsertModCache(cacheEntry)

		if _, exists := overrides[modID]; !exists {
			overrides[modID] = lua.ModOverride{
				Enabled:              true,
				ConfigurationOptions: defaults,
			}
		}
		added = append(added, modID)
	}

	if err := s.writeClusterOverrides(clusterID, shard, overrides); err != nil {
		return nil, err
	}
	return added, nil
}

// AddLocalModToCluster enables an already-uploaded local mod inside a cluster.
func (s *ModService) AddLocalModToCluster(clusterID, shard, modID, locale string) error {
	if !s.fs.ClusterExists(clusterID) {
		return db.ErrNotFound
	}
	if s.findModInfoPath(modID) == "" {
		return apperr.Newf(apperr.CodeLocalModNotFound, map[string]any{"mod_id": modID}, "local mod %q not found on disk", modID)
	}

	// Ensure mirrored into dst-server/mods/<modID>
	srcDir := filepath.Join(s.localModsDir, modID)
	dstDir := filepath.Join(s.dstModsDir(), modID)
	if _, err := os.Stat(srcDir); err == nil {
		if _, err := os.Stat(dstDir); err != nil {
			_ = copyDirSimple(srcDir, dstDir)
		}
	}

	raw, err := s.fs.ReadModOverrides(clusterID, shard)
	if err != nil {
		return err
	}
	overrides, err := lua.ParseModOverrides(raw)
	if err != nil {
		overrides = map[string]lua.ModOverride{}
	}

	_, info, _, _ := s.inspectMod(modID, locale)
	defaults := map[string]any{}
	if info != nil {
		for _, opt := range info.ConfigurationOptions {
			if opt.Default != nil {
				defaults[opt.Name] = opt.Default
			}
		}
	}

	overrides[modID] = lua.ModOverride{
		Enabled:              true,
		ConfigurationOptions: defaults,
	}
	return s.writeClusterOverrides(clusterID, shard, overrides)
}

func (s *ModService) SaveClusterMod(clusterID, shard, modID string, enabled bool, configOpts map[string]any) error {
	raw, err := s.fs.ReadModOverrides(clusterID, shard)
	if err != nil {
		return err
	}
	overrides, err := lua.ParseModOverrides(raw)
	if err != nil {
		return err
	}
	if configOpts == nil {
		configOpts = map[string]any{}
	}
	overrides[modID] = lua.ModOverride{
		Enabled:              enabled,
		ConfigurationOptions: configOpts,
	}
	return s.writeClusterOverrides(clusterID, shard, overrides)
}

func (s *ModService) RemoveClusterMod(clusterID, shard, modID string) error {
	raw, err := s.fs.ReadModOverrides(clusterID, shard)
	if err != nil {
		return err
	}
	overrides, err := lua.ParseModOverrides(raw)
	if err != nil {
		return err
	}
	delete(overrides, modID)
	return s.writeClusterOverrides(clusterID, shard, overrides)
}

func (s *ModService) SaveRawModOverrides(clusterID, shard, rawLua string) error {
	// Validate syntax before writing
	if _, err := lua.ParseModOverrides(rawLua); err != nil {
		return err
	}
	if err := s.fs.WriteModOverrides(clusterID, shard, rawLua); err != nil {
		return err
	}
	meta, err := s.store.GetClusterMeta(clusterID)
	if err == nil && meta.ModSyncEnabled && meta.CavesEnabled {
		sibling := "caves"
		if strings.ToLower(shard) == "caves" {
			sibling = "master"
		}
		_ = s.fs.WriteModOverrides(clusterID, sibling, rawLua)
	}
	return s.SyncDedicatedServerModsSetup()
}

func (s *ModService) SetModSync(clusterID string, enabled bool) error {
	meta, err := s.store.GetClusterMeta(clusterID)
	if err != nil {
		return err
	}
	meta.ModSyncEnabled = enabled
	if err := s.store.UpsertClusterMeta(meta); err != nil {
		return err
	}
	if enabled && meta.CavesEnabled {
		if masterRaw, err := s.fs.ReadModOverrides(clusterID, "master"); err == nil {
			_ = s.fs.WriteModOverrides(clusterID, "caves", masterRaw)
		}
	}
	return nil
}

func (s *ModService) RedownloadWorkshopMod(ctx context.Context, modID, locale string) error {
	if !strings.HasPrefix(modID, "workshop-") {
		return apperr.New(apperr.CodeOnlyWorkshopDownloadable, "only Steam Workshop mods can be downloaded via SteamCMD")
	}
	wid := strings.TrimPrefix(modID, "workshop-")
	if _, err := s.steamMgr.DownloadWorkshopMod(ctx, wid); err != nil {
		return err
	}
	downloaded, info, cache, parseErr := s.inspectMod(modID, locale)
	if !downloaded {
		return apperr.Newf(apperr.CodeWorkshopDownloadMissing, map[string]any{"path": filepath.Join(s.dstModsDir(), modID, "modinfo.lua")}, "modinfo.lua not found after downloading %s", modID)
	}
	if parseErr != nil {
		return parseErr
	}
	if info != nil {
		if cache == nil {
			cache = &db.ModCache{ModID: modID, WorkshopID: wid}
		}
		if cache.PreviewURL == "" {
			if detailMap := s.fetchWorkshopDetails(ctx, []string{wid}); detailMap != nil {
				if d, ok := detailMap[wid]; ok {
					mergeWorkshopDetailIntoCache(cache, d, false)
				}
			}
		}
		if info.Name != "" {
			cache.Name = info.Name
		}
		cache.Author = info.Author
		cache.Version = info.Version
		if info.Description != "" {
			cache.Description = info.Description
		}
		cache.ClientOnly = info.ClientOnlyMod
		_ = s.store.UpsertModCache(cache)
	}
	return nil
}

// SyncDedicatedServerModsSetup scans all clusters and generates dst-server/mods/dedicated_server_mods_setup.lua.
func (s *ModService) SyncDedicatedServerModsSetup() error {
	ids, err := s.fs.ListClusterIDs()
	if err != nil {
		return err
	}
	workshopSet := make(map[string]struct{})
	for _, cid := range ids {
		for _, sh := range []string{"master", "caves"} {
			raw, err := s.fs.ReadModOverrides(cid, sh)
			if err != nil {
				continue
			}
			ovs, err := lua.ParseModOverrides(raw)
			if err != nil {
				continue
			}
			for modID := range ovs {
				if strings.HasPrefix(modID, "workshop-") {
					wid := strings.TrimPrefix(modID, "workshop-")
					if wid != "" {
						workshopSet[wid] = struct{}{}
					}
				}
			}
		}
	}

	sorted := make([]string, 0, len(workshopSet))
	for wid := range workshopSet {
		sorted = append(sorted, wid)
	}
	sort.Strings(sorted)

	var sb strings.Builder
	sb.WriteString("-- Auto-generated by Walrus. Do not edit manually.\n")
	for _, wid := range sorted {
		fmt.Fprintf(&sb, "ServerModSetup(\"%s\")\n", wid)
	}

	setupPath := filepath.Join(s.dstModsDir(), "dedicated_server_mods_setup.lua")
	return dst.WriteFileAtomic(setupPath, []byte(sb.String()), 0o644)
}

// --- Local / Private Mod Upload & Global Library ---

var safeSlugRe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func (s *ModService) UploadLocalModZip(filename string, r io.ReaderAt, size int64, locale string) (*db.ModCache, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, apperr.WrapDetail(apperr.CodeInvalidZipArchive, err, "invalid zip archive: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "dst-local-mod-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	cleanDest := filepath.Clean(tmpDir) + string(os.PathSeparator)
	for _, f := range zr.File {
		target := filepath.Join(tmpDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) && filepath.Clean(target) != filepath.Clean(tmpDir) {
			return nil, apperr.Newf(apperr.CodeIllegalZipPath, map[string]any{"path": f.Name}, "illegal path in mod zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = rc.Close()
			return nil, err
		}
		_, copyErr := io.Copy(out, rc)
		_ = rc.Close()
		_ = out.Close()
		if copyErr != nil {
			return nil, copyErr
		}
	}

	// Locate modinfo.lua either at root or inside a single top-level folder.
	modRoot := tmpDir
	if _, err := os.Stat(filepath.Join(tmpDir, "modinfo.lua")); err != nil {
		entries, _ := os.ReadDir(tmpDir)
		found := false
		for _, e := range entries {
			if e.IsDir() {
				cand := filepath.Join(tmpDir, e.Name())
				if _, statErr := os.Stat(filepath.Join(cand, "modinfo.lua")); statErr == nil {
					modRoot = cand
					found = true
					break
				}
			}
		}
		if !found {
			return nil, apperr.New(apperr.CodeModZipMissingModInfo, "zip archive does not contain modinfo.lua at root or top-level folder")
		}
	}

	rawInfo, err := os.ReadFile(filepath.Join(modRoot, "modinfo.lua"))
	if err != nil {
		return nil, err
	}
	parsed, err := lua.ParseModInfo(string(rawInfo), locale, filename)
	if err != nil {
		return nil, apperr.WrapDetail(apperr.CodeModInfoParseFailed, err, "failed to parse modinfo.lua: %v", err)
	}

	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	slug := strings.Trim(safeSlugRe.ReplaceAllString(base, "_"), "_")
	if slug == "" {
		slug = "custom_mod"
	}
	if strings.HasPrefix(slug, "workshop-") {
		slug = "local-" + slug
	} else if !strings.HasPrefix(slug, "local-") {
		slug = "local-" + slug
	}

	finalLocalDir := filepath.Join(s.localModsDir, slug)
	finalDSTDir := filepath.Join(s.dstModsDir(), slug)
	_ = os.RemoveAll(finalLocalDir)
	_ = os.RemoveAll(finalDSTDir)
	if err := copyDirSimple(modRoot, finalLocalDir); err != nil {
		return nil, err
	}
	if err := copyDirSimple(modRoot, finalDSTDir); err != nil {
		return nil, err
	}

	rec := &db.ModCache{
		ModID:       slug,
		IsLocal:     true,
		Name:        parsed.Name,
		Author:      parsed.Author,
		Version:     parsed.Version,
		Description: parsed.Description,
		ClientOnly:  parsed.ClientOnlyMod,
		ServerTags:  strings.Join(parsed.ServerFilterTags, ","),
	}
	if rec.Name == "" {
		rec.Name = slug
	}
	if err := s.store.UpsertModCache(rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *ModService) ListLibraryMods() ([]db.ModCache, error) {
	list, err := s.store.ListModCache()
	if err != nil {
		return nil, err
	}
	if list == nil {
		return []db.ModCache{}, nil
	}
	return list, nil
}

func (s *ModService) DeleteLocalMod(modID string) error {
	if strings.HasPrefix(modID, "workshop-") || strings.Contains(modID, "/") || strings.Contains(modID, "\\") || strings.Contains(modID, "..") {
		return apperr.New(apperr.CodeOnlyLocalModDeletable, "only local mods can be deleted from the local repository")
	}
	_ = os.RemoveAll(filepath.Join(s.localModsDir, modID))
	_ = os.RemoveAll(filepath.Join(s.dstModsDir(), modID))
	return s.store.DeleteModCache(modID)
}

func (s *ModService) SearchWorkshop(ctx context.Context, query string, page int) ([]steam.WorkshopItem, int, bool, error) {
	st, _ := s.store.GetSystemSettings(true)
	apiKey := ""
	if st != nil {
		apiKey = st.SteamAPIKey
	}
	return s.workshop.SearchWorkshop(ctx, apiKey, query, page)
}

// SetProxy updates the outbound network proxy used by the Workshop client.
func (s *ModService) SetProxy(proxyURL string) error {
	if err := s.workshop.SetProxy(proxyURL); err != nil {
		return err
	}
	s.previewRetryMu.Lock()
	s.previewRetryAfter = make(map[string]time.Time)
	s.previewRetryMu.Unlock()
	return nil
}

func copyDirSimple(src, dstDir string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
