package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"walrus/internal/apperr"
	"walrus/internal/db"
	"walrus/internal/dst"
	"walrus/internal/lua"
	"walrus/internal/runtime"
)

type ClusterSummary struct {
	ClusterID       string                   `json:"cluster_id"`
	ClusterName     string                   `json:"cluster_name"`
	Description     string                   `json:"description"`
	ClusterPassword string                   `json:"cluster_password,omitempty"`
	GameMode        string                   `json:"game_mode"`
	MaxPlayers     int                      `json:"max_players"`
	CavesEnabled   bool                     `json:"caves_enabled"`
	AutoRestart    bool                     `json:"auto_restart"`
	MasterPort     int                      `json:"master_port"`
	CavesPort      int                      `json:"caves_port,omitempty"`
	MasterStatus   runtime.ShardStatus      `json:"master_status"`
	CavesStatus    *runtime.ShardStatus     `json:"caves_status,omitempty"`
	WorldInfo      runtime.WorldRuntimeInfo `json:"world_info"`
	EffectiveRole  db.Role                  `json:"effective_role"`
	HasToken       bool                     `json:"has_token"`
}

type ClusterDetail struct {
	ClusterID          string                   `json:"cluster_id"`
	Meta               db.ClusterMeta           `json:"meta"`
	ClusterINI         dst.ClusterINI           `json:"cluster_ini"`
	MasterServerINI    dst.ServerINI            `json:"master_server_ini"`
	CavesServerINI     *dst.ServerINI           `json:"caves_server_ini,omitempty"`
	MaskedClusterToken string                   `json:"masked_cluster_token"`
	HasClusterToken    bool                     `json:"has_cluster_token"`
	UsingDefaultToken  bool                     `json:"using_default_token"`
	MasterStatus       runtime.ShardStatus      `json:"master_status"`
	CavesStatus        *runtime.ShardStatus     `json:"caves_status,omitempty"`
	WorldInfo          runtime.WorldRuntimeInfo `json:"world_info"`
}

type CreateClusterRequest struct {
	ClusterID          string         `json:"cluster_id" binding:"required"`
	ClusterName        string         `json:"cluster_name" binding:"required"`
	ClusterDescription string         `json:"cluster_description"`
	ClusterPassword    string         `json:"cluster_password"`
	GameMode           string         `json:"game_mode"`
	MaxPlayers         int            `json:"max_players"`
	PVP                bool           `json:"pvp"`
	PauseWhenEmpty     *bool          `json:"pause_when_empty"`
	ClusterIntention   string         `json:"cluster_intention"`
	OfflineCluster     bool           `json:"offline_cluster"`
	LanOnlyCluster     bool           `json:"lan_only_cluster"`
	CavesEnabled       bool           `json:"caves_enabled"`
	MasterPreset       string         `json:"master_preset"`
	CavesPreset        string         `json:"caves_preset"`
	MasterOverrides    map[string]any `json:"master_overrides"`
	CavesOverrides     map[string]any `json:"caves_overrides"`
	ClusterToken       string         `json:"cluster_token"`
	AutoRestart        *bool          `json:"auto_restart"`
}

type UpdateClusterConfigRequest struct {
	ClusterINI      *dst.ClusterINI `json:"cluster_ini"`
	MasterServerINI *dst.ServerINI  `json:"master_server_ini"`
	CavesServerINI  *dst.ServerINI  `json:"caves_server_ini"`
	CavesEnabled    *bool           `json:"caves_enabled"`
	AutoRestart     *bool           `json:"auto_restart"`
	ModSyncEnabled  *bool           `json:"mod_sync_enabled"`
	ClusterToken    *string         `json:"cluster_token"` // nil = keep unchanged; "" = clear to fall back to default
}

type ClusterService struct {
	store  *db.Store
	fs     *dst.FS
	driver runtime.Driver
	modSvc *ModService
}

func NewClusterService(store *db.Store, fs *dst.FS, driver runtime.Driver) *ClusterService {
	return &ClusterService{store: store, fs: fs, driver: driver}
}

func (s *ClusterService) SetModService(modSvc *ModService) {
	s.modSvc = modSvc
}

func (s *ClusterService) ListClusters(u *db.User) ([]ClusterSummary, error) {
	ids, err := s.fs.ListClusterIDs()
	if err != nil {
		return nil, err
	}
	metas, err := s.store.ListClustersMeta()
	if err != nil {
		return nil, err
	}
	sysSettings, _ := s.store.GetSystemSettings(false)
	hasGlobalToken := sysSettings != nil && sysSettings.HasDefaultToken

	out := make([]ClusterSummary, 0, len(ids))
	for _, id := range ids {
		effRole, err := s.store.EffectiveClusterRole(u, id)
		if err != nil || effRole == "" {
			continue
		}
		ci, err := s.fs.ReadClusterINI(id)
		if err != nil {
			continue
		}
		var masterPort int
		if ms, err := s.fs.ReadServerINI(id, "master"); err == nil {
			masterPort = ms.ServerPort
		}

		meta, hasMeta := metas[id]
		cavesEnabled := s.fs.HasCavesDir(id)
		autoRestart := true
		if hasMeta {
			cavesEnabled = meta.CavesEnabled
			autoRestart = meta.AutoRestart
		}

		masterSt := s.driver.GetShardStatus(id, "master")
		var cavesSt *runtime.ShardStatus
		var cavesPort int
		if cavesEnabled {
			cs := s.driver.GetShardStatus(id, "caves")
			cavesSt = &cs
			if cini, err := s.fs.ReadServerINI(id, "caves"); err == nil {
				cavesPort = cini.ServerPort
			}
		}

		tok, _ := s.fs.ReadClusterToken(id)
		summary := ClusterSummary{
			ClusterID:       id,
			ClusterName:     ci.ClusterName,
			Description:     ci.ClusterDescription,
			ClusterPassword: ci.ClusterPassword,
			GameMode:        ci.GameMode,
			MaxPlayers:    ci.MaxPlayers,
			CavesEnabled:  cavesEnabled,
			AutoRestart:   autoRestart,
			MasterPort:    masterPort,
			CavesPort:     cavesPort,
			MasterStatus:  masterSt,
			CavesStatus:   cavesSt,
			WorldInfo:     s.driver.GetWorldInfo(id),
			EffectiveRole: effRole,
			HasToken:      tok != "" || hasGlobalToken,
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *ClusterService) GetCluster(clusterID string) (*ClusterDetail, error) {
	if !s.fs.ClusterExists(clusterID) {
		return nil, db.ErrNotFound
	}
	ci, err := s.fs.ReadClusterINI(clusterID)
	if err != nil {
		return nil, err
	}
	ms, err := s.fs.ReadServerINI(clusterID, "master")
	if err != nil {
		return nil, err
	}

	meta, err := s.store.GetClusterMeta(clusterID)
	if errors.Is(err, db.ErrNotFound) {
		meta = &db.ClusterMeta{
			ClusterID:      clusterID,
			DisplayName:    ci.ClusterName,
			Description:    ci.ClusterDescription,
			CavesEnabled:   s.fs.HasCavesDir(clusterID),
			ModSyncEnabled: true,
			AutoRestart:    true,
		}
		_ = s.store.UpsertClusterMeta(meta)
	} else if err != nil {
		return nil, err
	}

	var csINI *dst.ServerINI
	var cavesSt *runtime.ShardStatus
	if meta.CavesEnabled {
		c, _ := s.fs.ReadServerINI(clusterID, "caves")
		csINI = c
		st := s.driver.GetShardStatus(clusterID, "caves")
		cavesSt = &st
	}

	tok, _ := s.fs.ReadClusterToken(clusterID)
	sysSettings, _ := s.store.GetSystemSettings(false)
	usingDefault := tok == "" && sysSettings != nil && sysSettings.HasDefaultToken
	masked := db.MaskSecret(tok)
	if usingDefault {
		masked = sysSettings.DefaultClusterToken
	}

	return &ClusterDetail{
		ClusterID:          clusterID,
		Meta:               *meta,
		ClusterINI:         *ci,
		MasterServerINI:    *ms,
		CavesServerINI:     csINI,
		MaskedClusterToken: masked,
		HasClusterToken:    tok != "" || usingDefault,
		UsingDefaultToken:  usingDefault,
		MasterStatus:       s.driver.GetShardStatus(clusterID, "master"),
		CavesStatus:        cavesSt,
		WorldInfo:          s.driver.GetWorldInfo(clusterID),
	}, nil
}

func (s *ClusterService) PreviewNextPorts() (*dst.PortSet, error) {
	st, err := s.store.GetSystemSettings(false)
	if err != nil {
		return nil, err
	}
	return s.fs.AllocatePorts(st.PortPoolStart, st.PortPoolEnd)
}

func (s *ClusterService) CreateCluster(req CreateClusterRequest) (*ClusterDetail, error) {
	req.ClusterID = strings.TrimSpace(req.ClusterID)
	if err := dst.ValidateClusterID(req.ClusterID); err != nil {
		return nil, err
	}
	if s.fs.ClusterExists(req.ClusterID) {
		return nil, apperr.Newf(apperr.CodeClusterAlreadyExists, map[string]any{"cluster_id": req.ClusterID}, "cluster %q already exists", req.ClusterID)
	}

	st, err := s.store.GetSystemSettings(true)
	if err != nil {
		return nil, err
	}
	ports, err := s.fs.AllocatePorts(st.PortPoolStart, st.PortPoolEnd)
	if err != nil {
		return nil, err
	}

	ci := dst.DefaultClusterINI(strings.TrimSpace(req.ClusterName), ports.ClusterMasterPort, req.CavesEnabled)
	if req.ClusterDescription != "" {
		ci.ClusterDescription = req.ClusterDescription
	}
	ci.ClusterPassword = req.ClusterPassword
	if req.GameMode != "" {
		ci.GameMode = req.GameMode
	}
	if req.MaxPlayers > 0 {
		ci.MaxPlayers = req.MaxPlayers
	}
	ci.PVP = req.PVP
	if req.PauseWhenEmpty != nil {
		ci.PauseWhenEmpty = *req.PauseWhenEmpty
	}
	if req.ClusterIntention != "" {
		ci.ClusterIntention = req.ClusterIntention
	}
	ci.OfflineCluster = req.OfflineCluster
	ci.LanOnlyCluster = req.LanOnlyCluster

	if err := s.fs.WriteClusterINI(req.ClusterID, &ci); err != nil {
		return nil, err
	}

	masterINI := &dst.ServerINI{
		ServerPort:         ports.MasterServerPort,
		IsMaster:           true,
		Name:               "Master",
		ID:                 "1",
		EncodeUserPath:     true,
		MasterServerPort:   ports.MasterSteamPort,
		AuthenticationPort: ports.MasterAuthPort,
	}
	if err := s.fs.WriteServerINI(req.ClusterID, "master", masterINI); err != nil {
		return nil, err
	}

	presetID := req.MasterPreset
	if presetID == "" {
		presetID = ci.GameMode
	}
	masterLua := dst.PresetLuaByID(presetID, "master")
	if len(req.MasterOverrides) > 0 {
		patched, err := lua.PatchLevelOverrides(masterLua, req.MasterOverrides, false)
		if err != nil {
			return nil, err
		}
		masterLua = patched
	}
	if err := s.fs.WriteLevelData(req.ClusterID, "master", masterLua); err != nil {
		return nil, err
	}
	if err := s.fs.WriteModOverrides(req.ClusterID, "master", dst.DefaultModOverridesLua); err != nil {
		return nil, err
	}

	if req.CavesEnabled {
		cavesINI := &dst.ServerINI{
			ServerPort:         ports.CavesServerPort,
			IsMaster:           false,
			Name:               "Caves",
			ID:                 "2",
			EncodeUserPath:     true,
			MasterServerPort:   ports.CavesSteamPort,
			AuthenticationPort: ports.CavesAuthPort,
		}
		if err := s.fs.WriteServerINI(req.ClusterID, "caves", cavesINI); err != nil {
			return nil, err
		}
		cavesPresetID := req.CavesPreset
		if cavesPresetID == "" {
			cavesPresetID = "caves_default"
		}
		cavesLua := dst.PresetLuaByID(cavesPresetID, "caves")
		if len(req.CavesOverrides) > 0 {
			patched, err := lua.PatchLevelOverrides(cavesLua, req.CavesOverrides, false)
			if err != nil {
				return nil, err
			}
			cavesLua = patched
		}
		if err := s.fs.WriteLevelData(req.ClusterID, "caves", cavesLua); err != nil {
			return nil, err
		}
		if err := s.fs.WriteModOverrides(req.ClusterID, "caves", dst.DefaultModOverridesLua); err != nil {
			return nil, err
		}
	}

	if strings.TrimSpace(req.ClusterToken) != "" {
		if err := s.fs.WriteClusterToken(req.ClusterID, req.ClusterToken); err != nil {
			return nil, err
		}
	}

	autoRestart := true
	if req.AutoRestart != nil {
		autoRestart = *req.AutoRestart
	}
	meta := &db.ClusterMeta{
		ClusterID:      req.ClusterID,
		DisplayName:    ci.ClusterName,
		Description:    ci.ClusterDescription,
		CavesEnabled:   req.CavesEnabled,
		ModSyncEnabled: true,
		AutoRestart:    autoRestart,
	}
	if err := s.store.UpsertClusterMeta(meta); err != nil {
		return nil, err
	}

	return s.GetCluster(req.ClusterID)
}

func (s *ClusterService) UpdateClusterConfig(clusterID string, req UpdateClusterConfigRequest) (*ClusterDetail, error) {
	detail, err := s.GetCluster(clusterID)
	if err != nil {
		return nil, err
	}

	ci := detail.ClusterINI
	if req.ClusterINI != nil {
		ci = *req.ClusterINI
	}
	ms := detail.MasterServerINI
	if req.MasterServerINI != nil {
		ms = *req.MasterServerINI
		ms.IsMaster = true
	}

	cavesEnabled := detail.Meta.CavesEnabled
	if req.CavesEnabled != nil {
		cavesEnabled = *req.CavesEnabled
	}
	ci.ShardEnabled = cavesEnabled

	var cs dst.ServerINI
	if detail.CavesServerINI != nil {
		cs = *detail.CavesServerINI
	} else if cavesEnabled {
		// Newly enabling Caves: allocate non-conflicting ports.
		st, _ := s.store.GetSystemSettings(false)
		ports, err := s.fs.AllocatePorts(st.PortPoolStart, st.PortPoolEnd)
		if err != nil {
			return nil, err
		}
		cs = dst.ServerINI{
			ServerPort:         ports.CavesServerPort,
			IsMaster:           false,
			Name:               "Caves",
			ID:                 "2",
			EncodeUserPath:     true,
			MasterServerPort:   ports.CavesSteamPort,
			AuthenticationPort: ports.CavesAuthPort,
		}
	}
	if req.CavesServerINI != nil {
		cs = *req.CavesServerINI
		cs.IsMaster = false
	}

	portCheck := dst.PortSet{
		MasterServerPort:  ms.ServerPort,
		MasterAuthPort:    ms.AuthenticationPort,
		MasterSteamPort:   ms.MasterServerPort,
		ClusterMasterPort: ci.MasterPort,
		CavesServerPort:   cs.ServerPort,
		CavesAuthPort:     cs.AuthenticationPort,
		CavesSteamPort:    cs.MasterServerPort,
	}
	if err := s.fs.ValidatePortConflicts(clusterID, portCheck, cavesEnabled); err != nil {
		return nil, err
	}

	if err := s.fs.WriteClusterINI(clusterID, &ci); err != nil {
		return nil, err
	}
	if err := s.fs.WriteServerINI(clusterID, "master", &ms); err != nil {
		return nil, err
	}
	if cavesEnabled {
		if err := s.fs.WriteServerINI(clusterID, "caves", &cs); err != nil {
			return nil, err
		}
		if !s.fs.HasCavesDir(clusterID) || detail.CavesServerINI == nil {
			// Ensure Caves leveldataoverride & modoverrides exist if missing.
			if cur, err := s.fs.ReadLevelData(clusterID, "caves"); err == nil {
				_ = s.fs.WriteLevelData(clusterID, "caves", cur)
			}
			if curM, err := s.fs.ReadModOverrides(clusterID, "caves"); err == nil {
				_ = s.fs.WriteModOverrides(clusterID, "caves", curM)
			}
		}
	}

	if req.ClusterToken != nil {
		tok := strings.TrimSpace(*req.ClusterToken)
		if tok == "" {
			dir, _ := s.fs.ClusterDir(clusterID)
			_ = os.Remove(dir + "/cluster_token.txt")
		} else {
			if err := s.fs.WriteClusterToken(clusterID, tok); err != nil {
				return nil, err
			}
		}
	}

	meta := detail.Meta
	meta.DisplayName = ci.ClusterName
	meta.Description = ci.ClusterDescription
	meta.CavesEnabled = cavesEnabled
	if req.AutoRestart != nil {
		meta.AutoRestart = *req.AutoRestart
	}
	if req.ModSyncEnabled != nil {
		meta.ModSyncEnabled = *req.ModSyncEnabled
	}
	if err := s.store.UpsertClusterMeta(&meta); err != nil {
		return nil, err
	}

	return s.GetCluster(clusterID)
}

func (s *ClusterService) ReadWorldLua(clusterID, shard string) (string, error) {
	return s.fs.ReadLevelData(clusterID, shard)
}

type WorldConfigView struct {
	Shard  string                 `json:"shard"`
	Lua    string                 `json:"lua"`
	Parsed *lua.ParsedLevelData   `json:"parsed,omitempty"`
	Schema []dst.WorldOptionGroup `json:"schema"`
}

func (s *ClusterService) ReadWorldConfig(clusterID, shard string) (*WorldConfigView, error) {
	raw, err := s.fs.ReadLevelData(clusterID, shard)
	if err != nil {
		return nil, err
	}
	parsed, _ := lua.ParseLevelData(raw)
	return &WorldConfigView{
		Shard:  strings.ToLower(shard),
		Lua:    raw,
		Parsed: parsed,
		Schema: dst.WorldSchema(strings.ToLower(shard)),
	}, nil
}

func (s *ClusterService) SaveWorldOverrides(clusterID, shard string, overrides map[string]any) (string, error) {
	raw, err := s.fs.ReadLevelData(clusterID, shard)
	if err != nil {
		return "", err
	}
	patched, err := lua.PatchLevelOverrides(raw, overrides, false)
	if err != nil {
		return "", err
	}
	if err := s.fs.WriteLevelData(clusterID, shard, patched); err != nil {
		return "", err
	}
	return patched, nil
}

func (s *ClusterService) WriteWorldLua(clusterID, shard, content string) error {
	if err := lua.ValidateLevelLua(content); err != nil {
		if _, ok := err.(*apperr.Error); ok {
			return err
		}
		return apperr.WrapDetail(apperr.CodeLuaValidationFailed, err, "Lua syntax validation failed: %v", err)
	}
	return s.fs.WriteLevelData(clusterID, shard, content)
}

// --- Player Lists & Chat ---

type PlayerLists struct {
	AdminList []string           `json:"adminlist"`
	WhiteList []string           `json:"whitelist"`
	BlockList []string           `json:"blocklist"`
	History   []db.PlayerHistory `json:"history"`
}

func (s *ClusterService) GetPlayerLists(clusterID string) (*PlayerLists, error) {
	if !s.fs.ClusterExists(clusterID) {
		return nil, db.ErrNotFound
	}
	admins, err := s.fs.ReadListFile(clusterID, "adminlist.txt")
	if err != nil {
		return nil, err
	}
	whites, err := s.fs.ReadListFile(clusterID, "whitelist.txt")
	if err != nil {
		return nil, err
	}
	blocks, err := s.fs.ReadListFile(clusterID, "blocklist.txt")
	if err != nil {
		return nil, err
	}
	hist, _ := s.store.ListPlayerHistory(clusterID, 200)
	if hist == nil {
		hist = []db.PlayerHistory{}
	}
	return &PlayerLists{
		AdminList: admins,
		WhiteList: whites,
		BlockList: blocks,
		History:   hist,
	}, nil
}

func (s *ClusterService) SavePlayerLists(clusterID string, admins, whites, blocks []string) error {
	if !s.fs.ClusterExists(clusterID) {
		return db.ErrNotFound
	}
	if err := s.fs.WriteListFile(clusterID, "adminlist.txt", admins); err != nil {
		return err
	}
	if err := s.fs.WriteListFile(clusterID, "whitelist.txt", whites); err != nil {
		return err
	}
	return s.fs.WriteListFile(clusterID, "blocklist.txt", blocks)
}

func (s *ClusterService) PlayerAction(clusterID, action, kuID string) error {
	kuID = strings.TrimSpace(kuID)
	if kuID == "" || strings.ContainsAny(kuID, "\"'\\\n\r") {
		return apperr.New(apperr.CodeInvalidPlayerKUID, "invalid player KU_ID")
	}
	switch action {
	case "kick":
		return s.driver.SendCommand(clusterID, "master", fmt.Sprintf(`TheNet:Kick("%s")`, kuID))
	case "ban":
		if err := s.driver.SendCommand(clusterID, "master", fmt.Sprintf(`TheNet:Ban("%s")`, kuID)); err != nil {
			return err
		}
		blocks, _ := s.fs.ReadListFile(clusterID, "blocklist.txt")
		blocks = append(blocks, kuID)
		return s.fs.WriteListFile(clusterID, "blocklist.txt", blocks)
	case "revive":
		cmd := fmt.Sprintf(`for _,p in ipairs(AllPlayers) do if p.userid=="%s" and p:HasTag("playerghost") then p:PushEvent("respawnfromghost") end end`, kuID)
		return s.driver.SendCommand(clusterID, "master", cmd)
	default:
		return apperr.Newf(apperr.CodeUnknownPlayerAction, map[string]any{"action": action}, "unknown player action: %s", action)
	}
}

func (s *ClusterService) ReadRecentChat(clusterID string, limit int) ([]runtime.ChatMessage, error) {
	return s.driver.ReadRecentChat(clusterID, limit)
}

func (s *ClusterService) TriggerWorldProbe(clusterID string) error {
	return s.driver.TriggerProbe(clusterID)
}

func (s *ClusterService) ensureTokenReady(clusterID string) error {
	tok, err := s.fs.ReadClusterToken(clusterID)
	if err != nil {
		return err
	}
	if tok != "" {
		return nil
	}
	st, err := s.store.GetSystemSettings(true)
	if err != nil {
		return err
	}
	if st.DefaultClusterToken != "" {
		return s.fs.WriteClusterToken(clusterID, st.DefaultClusterToken)
	}
	if s.driver.Mode() == "mock" {
		// Mock mode does not require a real Klei token.
		return nil
	}
	return apperr.New(apperr.CodeClusterTokenMissing, "cluster_token.txt is empty and no global default Klei token is configured")
}

func (s *ClusterService) ensureLevelDataReady(clusterID, shard string) {
	if !s.fs.HasLevelData(clusterID, shard) {
		return
	}
	raw, err := s.fs.ReadLevelData(clusterID, shard)
	if err != nil {
		return
	}
	patched, changed, err := lua.EnsureLevelDataDefaults(raw, shard)
	if err != nil || !changed {
		return
	}
	_ = s.fs.WriteLevelData(clusterID, shard, patched)
}

func (s *ClusterService) StartShard(ctx context.Context, clusterID, shard string) error {
	detail, err := s.GetCluster(clusterID)
	if err != nil {
		return err
	}
	if strings.ToLower(shard) == "caves" && !detail.Meta.CavesEnabled {
		return apperr.New(apperr.CodeCavesDisabled, "caves shard is disabled for this cluster")
	}
	if err := s.ensureTokenReady(clusterID); err != nil {
		return err
	}
	s.ensureLevelDataReady(clusterID, shard)
	st, _ := s.store.GetSystemSettings(false)
	arch := "64"
	if st != nil && st.DSTArch != "" {
		arch = st.DSTArch
	}
	opts := runtime.StartOptions{
		AutoRestart: detail.Meta.AutoRestart,
		Arch:        arch,
	}
	if s.modSvc != nil {
		modSvc := s.modSvc
		normShard := strings.ToLower(shard)
		opts.Prepare = func(prepCtx context.Context, logFn func(level, msg string)) error {
			return modSvc.EnsureShardModsReady(prepCtx, clusterID, normShard, logFn)
		}
	}
	return s.driver.StartShard(ctx, clusterID, shard, opts)
}

func (s *ClusterService) StopShard(ctx context.Context, clusterID, shard string) error {
	return s.driver.StopShard(ctx, clusterID, shard)
}

func (s *ClusterService) StartCluster(ctx context.Context, clusterID string) error {
	detail, err := s.GetCluster(clusterID)
	if err != nil {
		return err
	}
	if err := s.StartShard(ctx, clusterID, "master"); err != nil {
		return err
	}
	if detail.Meta.CavesEnabled {
		_ = s.StartShard(ctx, clusterID, "caves")
	}
	return nil
}

func (s *ClusterService) StopCluster(ctx context.Context, clusterID string) error {
	detail, err := s.GetCluster(clusterID)
	if err != nil {
		return err
	}
	if detail.Meta.CavesEnabled {
		_ = s.driver.StopShard(ctx, clusterID, "caves")
	}
	return s.driver.StopShard(ctx, clusterID, "master")
}

func (s *ClusterService) RestartCluster(ctx context.Context, clusterID string) error {
	if err := s.StopCluster(ctx, clusterID); err != nil {
		return err
	}
	return s.StartCluster(ctx, clusterID)
}

func (s *ClusterService) SendCommand(clusterID, shard, command string) error {
	return s.driver.SendCommand(clusterID, shard, command)
}

func (s *ClusterService) DeleteCluster(ctx context.Context, clusterID string) error {
	if err := dst.ValidateClusterID(clusterID); err != nil {
		return err
	}
	_ = s.driver.StopShard(ctx, clusterID, "caves")
	_ = s.driver.StopShard(ctx, clusterID, "master")

	dir, err := s.fs.ClusterDir(clusterID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return s.store.DeleteClusterMeta(clusterID)
}
