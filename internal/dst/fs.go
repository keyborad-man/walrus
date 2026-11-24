package dst

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/ini.v1"

	"walrus/internal/apperr"
)

var (
	clusterIDRe      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{1,47}$`)
	ErrInvalidID     = apperr.New(apperr.CodeInvalidClusterID, "cluster id must be 2-48 chars using letters, digits, '-' or '_'")
	ErrPortExhausted = apperr.New(apperr.CodePortExhausted, "no contiguous free port block available in configured port pool")
)

func ValidateClusterID(id string) error {
	if !clusterIDRe.MatchString(id) {
		return ErrInvalidID
	}
	return nil
}

func RandomClusterKey() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "dst-" + hex.EncodeToString(b)
}

func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// FS provides disk-backed Source-of-Truth operations on the DST clusters root directory.
type FS struct {
	ClustersRoot string
}

func NewFS(clustersRoot string) *FS {
	return &FS{ClustersRoot: clustersRoot}
}

func (f *FS) ClusterDir(clusterID string) (string, error) {
	if err := ValidateClusterID(clusterID); err != nil {
		return "", err
	}
	return filepath.Join(f.ClustersRoot, clusterID), nil
}

func (f *FS) ShardDir(clusterID, shard string) (string, error) {
	dir, err := f.ClusterDir(clusterID)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(shard) {
	case "master":
		return filepath.Join(dir, "Master"), nil
	case "caves":
		return filepath.Join(dir, "Caves"), nil
	default:
		return "", apperr.Newf(apperr.CodeInvalidShard, map[string]any{"shard": shard}, "invalid shard %q: must be master or caves", shard)
	}
}

func (f *FS) ListClusterIDs() ([]string, error) {
	entries, err := os.ReadDir(f.ClustersRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if ValidateClusterID(e.Name()) != nil {
			continue
		}
		// Must have cluster.ini or Master dir to count as a cluster.
		cPath := filepath.Join(f.ClustersRoot, e.Name())
		if exists(filepath.Join(cPath, "cluster.ini")) || exists(filepath.Join(cPath, "Master")) {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (f *FS) ClusterExists(clusterID string) bool {
	dir, err := f.ClusterDir(clusterID)
	if err != nil {
		return false
	}
	return exists(dir)
}

func (f *FS) HasCavesDir(clusterID string) bool {
	dir, err := f.ShardDir(clusterID, "caves")
	if err != nil {
		return false
	}
	return exists(filepath.Join(dir, "server.ini"))
}

// --- cluster.ini ---

func DefaultClusterINI(name string, masterPort int, cavesEnabled bool) ClusterINI {
	return ClusterINI{
		GameMode:           "endless",
		MaxPlayers:         6,
		PVP:                false,
		PauseWhenEmpty:     true,
		ClusterName:        name,
		ClusterDescription: "Managed by Walrus",
		ClusterPassword:    "",
		ClusterIntention:   "cooperative",
		ClusterLanguage:    "zh",
		LanOnlyCluster:     false,
		OfflineCluster:     false,
		AutosaverEnabled:   true,
		TickRate:           15,
		WhitelistSlots:     0,
		ConsoleEnabled:     true,
		MaxSnapshots:       6,
		ShardEnabled:       cavesEnabled,
		BindIP:             "127.0.0.1",
		MasterIP:           "127.0.0.1",
		MasterPort:         masterPort,
		ClusterKey:         RandomClusterKey(),
	}
}

func (f *FS) ReadClusterINI(clusterID string) (*ClusterINI, error) {
	dir, err := f.ClusterDir(clusterID)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(dir, "cluster.ini")
	cfg := DefaultClusterINI(clusterID, 10888, true)

	file, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, p)
	if errors.Is(err, os.ErrNotExist) {
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cluster.ini: %w", err)
	}

	gp := file.Section("GAMEPLAY")
	cfg.GameMode = gp.Key("game_mode").MustString(cfg.GameMode)
	cfg.MaxPlayers = gp.Key("max_players").MustInt(cfg.MaxPlayers)
	cfg.PVP = gp.Key("pvp").MustBool(cfg.PVP)
	cfg.PauseWhenEmpty = gp.Key("pause_when_empty").MustBool(cfg.PauseWhenEmpty)

	nw := file.Section("NETWORK")
	cfg.ClusterName = nw.Key("cluster_name").MustString(cfg.ClusterName)
	cfg.ClusterDescription = nw.Key("cluster_description").MustString(cfg.ClusterDescription)
	cfg.ClusterPassword = nw.Key("cluster_password").MustString(cfg.ClusterPassword)
	cfg.ClusterIntention = nw.Key("cluster_intention").MustString(cfg.ClusterIntention)
	cfg.ClusterLanguage = nw.Key("cluster_language").MustString(cfg.ClusterLanguage)
	cfg.LanOnlyCluster = nw.Key("lan_only_cluster").MustBool(cfg.LanOnlyCluster)
	cfg.OfflineCluster = nw.Key("offline_cluster").MustBool(cfg.OfflineCluster)
	cfg.AutosaverEnabled = nw.Key("autosaver_enabled").MustBool(cfg.AutosaverEnabled)
	cfg.TickRate = nw.Key("tick_rate").MustInt(cfg.TickRate)
	cfg.WhitelistSlots = nw.Key("whitelist_slots").MustInt(cfg.WhitelistSlots)

	ms := file.Section("MISC")
	cfg.ConsoleEnabled = ms.Key("console_enabled").MustBool(cfg.ConsoleEnabled)
	cfg.MaxSnapshots = ms.Key("max_snapshots").MustInt(cfg.MaxSnapshots)

	sh := file.Section("SHARD")
	cfg.ShardEnabled = sh.Key("shard_enabled").MustBool(cfg.ShardEnabled)
	cfg.BindIP = sh.Key("bind_ip").MustString(cfg.BindIP)
	cfg.MasterIP = sh.Key("master_ip").MustString(cfg.MasterIP)
	cfg.MasterPort = sh.Key("master_port").MustInt(cfg.MasterPort)
	cfg.ClusterKey = sh.Key("cluster_key").MustString(cfg.ClusterKey)

	return &cfg, nil
}

func (f *FS) WriteClusterINI(clusterID string, c *ClusterINI) error {
	dir, err := f.ClusterDir(clusterID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(c.ClusterKey) == "" {
		c.ClusterKey = RandomClusterKey()
	}
	if strings.TrimSpace(c.BindIP) == "" {
		c.BindIP = "127.0.0.1"
	}
	if strings.TrimSpace(c.MasterIP) == "" {
		c.MasterIP = "127.0.0.1"
	}
	if c.MaxPlayers <= 0 {
		c.MaxPlayers = 6
	}
	if c.TickRate <= 0 {
		c.TickRate = 15
	}
	if c.MaxSnapshots <= 0 {
		c.MaxSnapshots = 6
	}

	var sb strings.Builder
	sb.WriteString("[GAMEPLAY]\n")
	fmt.Fprintf(&sb, "game_mode = %s\n", c.GameMode)
	fmt.Fprintf(&sb, "max_players = %d\n", c.MaxPlayers)
	fmt.Fprintf(&sb, "pvp = %t\n", c.PVP)
	fmt.Fprintf(&sb, "pause_when_empty = %t\n\n", c.PauseWhenEmpty)

	sb.WriteString("[NETWORK]\n")
	fmt.Fprintf(&sb, "cluster_name = %s\n", c.ClusterName)
	fmt.Fprintf(&sb, "cluster_description = %s\n", c.ClusterDescription)
	fmt.Fprintf(&sb, "cluster_password = %s\n", c.ClusterPassword)
	fmt.Fprintf(&sb, "cluster_intention = %s\n", c.ClusterIntention)
	fmt.Fprintf(&sb, "cluster_language = %s\n", c.ClusterLanguage)
	fmt.Fprintf(&sb, "lan_only_cluster = %t\n", c.LanOnlyCluster)
	fmt.Fprintf(&sb, "offline_cluster = %t\n", c.OfflineCluster)
	fmt.Fprintf(&sb, "autosaver_enabled = %t\n", c.AutosaverEnabled)
	fmt.Fprintf(&sb, "tick_rate = %d\n", c.TickRate)
	fmt.Fprintf(&sb, "whitelist_slots = %d\n\n", c.WhitelistSlots)

	sb.WriteString("[MISC]\n")
	fmt.Fprintf(&sb, "console_enabled = %t\n", c.ConsoleEnabled)
	fmt.Fprintf(&sb, "max_snapshots = %d\n\n", c.MaxSnapshots)

	sb.WriteString("[SHARD]\n")
	fmt.Fprintf(&sb, "shard_enabled = %t\n", c.ShardEnabled)
	fmt.Fprintf(&sb, "bind_ip = %s\n", c.BindIP)
	fmt.Fprintf(&sb, "master_ip = %s\n", c.MasterIP)
	fmt.Fprintf(&sb, "master_port = %d\n", c.MasterPort)
	fmt.Fprintf(&sb, "cluster_key = %s\n", c.ClusterKey)

	return WriteFileAtomic(filepath.Join(dir, "cluster.ini"), []byte(sb.String()), 0o644)
}

// --- server.ini ---

func (f *FS) ReadServerINI(clusterID, shard string) (*ServerINI, error) {
	dir, err := f.ShardDir(clusterID, shard)
	if err != nil {
		return nil, err
	}
	isMaster := strings.ToLower(shard) == "master"
	defName := "Master"
	defID := "1"
	if !isMaster {
		defName = "Caves"
		defID = "2"
	}
	cfg := ServerINI{
		ServerPort:         10999,
		IsMaster:           isMaster,
		Name:               defName,
		ID:                 defID,
		EncodeUserPath:     true,
		MasterServerPort:   27016,
		AuthenticationPort: 8766,
	}

	p := filepath.Join(dir, "server.ini")
	file, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, p)
	if errors.Is(err, os.ErrNotExist) {
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s/server.ini: %w", shard, err)
	}

	cfg.ServerPort = file.Section("NETWORK").Key("server_port").MustInt(cfg.ServerPort)
	cfg.IsMaster = file.Section("SHARD").Key("is_master").MustBool(cfg.IsMaster)
	cfg.Name = file.Section("SHARD").Key("name").MustString(cfg.Name)
	cfg.ID = file.Section("SHARD").Key("id").MustString(cfg.ID)
	cfg.EncodeUserPath = file.Section("ACCOUNT").Key("encode_user_path").MustBool(cfg.EncodeUserPath)
	cfg.MasterServerPort = file.Section("STEAM").Key("master_server_port").MustInt(cfg.MasterServerPort)
	cfg.AuthenticationPort = file.Section("STEAM").Key("authentication_port").MustInt(cfg.AuthenticationPort)
	return &cfg, nil
}

func (f *FS) WriteServerINI(clusterID, shard string, s *ServerINI) error {
	dir, err := f.ShardDir(clusterID, shard)
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("[NETWORK]\n")
	fmt.Fprintf(&sb, "server_port = %d\n\n", s.ServerPort)

	sb.WriteString("[SHARD]\n")
	fmt.Fprintf(&sb, "is_master = %t\n", s.IsMaster)
	if !s.IsMaster {
		fmt.Fprintf(&sb, "name = %s\n", s.Name)
		if s.ID != "" {
			fmt.Fprintf(&sb, "id = %s\n", s.ID)
		}
	}
	sb.WriteString("\n[ACCOUNT]\n")
	fmt.Fprintf(&sb, "encode_user_path = %t\n\n", s.EncodeUserPath)

	sb.WriteString("[STEAM]\n")
	fmt.Fprintf(&sb, "master_server_port = %d\n", s.MasterServerPort)
	fmt.Fprintf(&sb, "authentication_port = %d\n", s.AuthenticationPort)

	return WriteFileAtomic(filepath.Join(dir, "server.ini"), []byte(sb.String()), 0o644)
}

// --- cluster_token.txt, leveldataoverride.lua, modoverrides.lua, lists ---

func (f *FS) ReadClusterToken(clusterID string) (string, error) {
	dir, err := f.ClusterDir(clusterID)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(dir, "cluster_token.txt"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (f *FS) WriteClusterToken(clusterID, token string) error {
	dir, err := f.ClusterDir(clusterID)
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(dir, "cluster_token.txt"), []byte(strings.TrimSpace(token)+"\n"), 0o600)
}

func (f *FS) ReadLevelData(clusterID, shard string) (string, error) {
	dir, err := f.ShardDir(clusterID, shard)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(dir, "leveldataoverride.lua"))
	if errors.Is(err, os.ErrNotExist) {
		if strings.ToLower(shard) == "caves" {
			return PresetLuaByID("caves_default", "caves"), nil
		}
		return PresetLuaByID("endless", "master"), nil
	}
	return string(b), err
}

func (f *FS) WriteLevelData(clusterID, shard, content string) error {
	dir, err := f.ShardDir(clusterID, shard)
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(dir, "leveldataoverride.lua"), []byte(content), 0o644)
}

func (f *FS) ReadModOverrides(clusterID, shard string) (string, error) {
	dir, err := f.ShardDir(clusterID, shard)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(dir, "modoverrides.lua"))
	if errors.Is(err, os.ErrNotExist) {
		return DefaultModOverridesLua, nil
	}
	return string(b), err
}

func (f *FS) WriteModOverrides(clusterID, shard, content string) error {
	dir, err := f.ShardDir(clusterID, shard)
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(dir, "modoverrides.lua"), []byte(content), 0o644)
}

func (f *FS) ReadListFile(clusterID, filename string) ([]string, error) {
	dir, err := f.ClusterDir(clusterID)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(dir, filename))
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out, nil
}

func (f *FS) WriteListFile(clusterID, filename string, items []string) error {
	dir, err := f.ClusterDir(clusterID)
	if err != nil {
		return err
	}
	var clean []string
	seen := make(map[string]struct{})
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		clean = append(clean, item)
	}
	content := strings.Join(clean, "\n")
	if content != "" {
		content += "\n"
	}
	return WriteFileAtomic(filepath.Join(dir, filename), []byte(content), 0o644)
}

// --- Port Pool Allocation & Conflict Detection ---

// CollectUsedPorts returns a map of port -> clusterID for all clusters on disk (excluding skipClusterID).
func (f *FS) CollectUsedPorts(skipClusterID string) (map[int]string, error) {
	ids, err := f.ListClusterIDs()
	if err != nil {
		return nil, err
	}
	used := make(map[int]string)
	for _, id := range ids {
		if id == skipClusterID {
			continue
		}
		if ci, err := f.ReadClusterINI(id); err == nil && ci.MasterPort > 0 {
			used[ci.MasterPort] = id
		}
		if ms, err := f.ReadServerINI(id, "master"); err == nil {
			addPort(used, ms.ServerPort, id)
			addPort(used, ms.AuthenticationPort, id)
			addPort(used, ms.MasterServerPort, id)
		}
		if f.HasCavesDir(id) {
			if cs, err := f.ReadServerINI(id, "caves"); err == nil {
				addPort(used, cs.ServerPort, id)
				addPort(used, cs.AuthenticationPort, id)
				addPort(used, cs.MasterServerPort, id)
			}
		}
	}
	return used, nil
}

func addPort(m map[int]string, p int, id string) {
	if p > 0 {
		m[p] = id
	}
}

// AllocatePorts finds the first 7 unused ports within [poolStart, poolEnd].
func (f *FS) AllocatePorts(poolStart, poolEnd int) (*PortSet, error) {
	if poolStart < 1024 {
		poolStart = 10990
	}
	if poolEnd < poolStart+6 {
		poolEnd = poolStart + 70
	}
	used, err := f.CollectUsedPorts("")
	if err != nil {
		return nil, err
	}

	var free []int
	for p := poolStart; p <= poolEnd && len(free) < 7; p++ {
		if _, busy := used[p]; !busy {
			free = append(free, p)
		}
	}
	if len(free) < 7 {
		return nil, ErrPortExhausted
	}
	return &PortSet{
		MasterServerPort:  free[0],
		CavesServerPort:   free[1],
		MasterAuthPort:    free[2],
		CavesAuthPort:     free[3],
		MasterSteamPort:   free[4],
		CavesSteamPort:    free[5],
		ClusterMasterPort: free[6],
	}, nil
}

// ValidatePortConflicts checks that a cluster's ports have no internal duplicates or cross-cluster overlaps.
func (f *FS) ValidatePortConflicts(clusterID string, ports PortSet, cavesEnabled bool) error {
	used, err := f.CollectUsedPorts(clusterID)
	if err != nil {
		return err
	}
	checkPorts := []struct {
		label string
		port  int
	}{
		{"master.server_port", ports.MasterServerPort},
		{"master.authentication_port", ports.MasterAuthPort},
		{"master.steam_master_server_port", ports.MasterSteamPort},
		{"cluster.master_port", ports.ClusterMasterPort},
	}
	if cavesEnabled {
		checkPorts = append(checkPorts,
			struct {
				label string
				port  int
			}{"caves.server_port", ports.CavesServerPort},
			struct {
				label string
				port  int
			}{"caves.authentication_port", ports.CavesAuthPort},
			struct {
				label string
				port  int
			}{"caves.steam_master_server_port", ports.CavesSteamPort},
		)
	}

	seen := make(map[int]string)
	for _, item := range checkPorts {
		if item.port < 1024 || item.port > 65535 {
			return apperr.Newf(apperr.CodePortOutOfRange, map[string]any{"label": item.label, "port": item.port}, "%s (%d) must be in range 1024-65535", item.label, item.port)
		}
		if prev, dup := seen[item.port]; dup {
			return apperr.Newf(apperr.CodePortDuplicate, map[string]any{"port": item.port, "first": prev, "second": item.label}, "duplicate port %d between %s and %s", item.port, prev, item.label)
		}
		seen[item.port] = item.label
		if owner, busy := used[item.port]; busy {
			return apperr.Newf(apperr.CodePortConflict, map[string]any{"port": item.port, "label": item.label, "owner": owner}, "port %d (%s) is already used by cluster %q", item.port, item.label, owner)
		}
	}
	return nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
