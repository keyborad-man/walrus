package dst

// ClusterINI models DST's cluster.ini file.
type ClusterINI struct {
	// [GAMEPLAY]
	GameMode       string `json:"game_mode"`        // survival | endless | wilderness | relaxed
	MaxPlayers     int    `json:"max_players"`
	PVP            bool   `json:"pvp"`
	PauseWhenEmpty bool   `json:"pause_when_empty"`

	// [NETWORK]
	ClusterName        string `json:"cluster_name"`
	ClusterDescription string `json:"cluster_description"`
	ClusterPassword    string `json:"cluster_password"`
	ClusterIntention   string `json:"cluster_intention"` // cooperative | social | competitive | madness
	ClusterLanguage    string `json:"cluster_language"`  // zh | en
	LanOnlyCluster     bool   `json:"lan_only_cluster"`
	OfflineCluster     bool   `json:"offline_cluster"`
	AutosaverEnabled   bool   `json:"autosaver_enabled"`
	TickRate           int    `json:"tick_rate"`
	WhitelistSlots     int    `json:"whitelist_slots"`

	// [MISC]
	ConsoleEnabled bool `json:"console_enabled"`
	MaxSnapshots   int  `json:"max_snapshots"`

	// [SHARD]
	ShardEnabled bool   `json:"shard_enabled"`
	BindIP       string `json:"bind_ip"`
	MasterIP     string `json:"master_ip"`
	MasterPort   int    `json:"master_port"`
	ClusterKey   string `json:"cluster_key"`
}

// ServerINI models DST's Master/server.ini or Caves/server.ini.
type ServerINI struct {
	ServerPort         int    `json:"server_port"`
	IsMaster           bool   `json:"is_master"`
	Name               string `json:"name"`
	ID                 string `json:"id"`
	EncodeUserPath     bool   `json:"encode_user_path"`
	MasterServerPort   int    `json:"master_server_port"`
	AuthenticationPort int    `json:"authentication_port"`
}

// PortSet holds all non-conflicting ports assigned to one dual-shard cluster.
type PortSet struct {
	MasterServerPort int `json:"master_server_port"`
	CavesServerPort  int `json:"caves_server_port"`
	MasterAuthPort   int `json:"master_auth_port"`
	CavesAuthPort    int `json:"caves_auth_port"`
	MasterSteamPort  int `json:"master_steam_port"`
	CavesSteamPort   int `json:"caves_steam_port"`
	ClusterMasterPort int `json:"cluster_master_port"`
}

func (p PortSet) AllPorts() []int {
	return []int{
		p.MasterServerPort,
		p.CavesServerPort,
		p.MasterAuthPort,
		p.CavesAuthPort,
		p.MasterSteamPort,
		p.CavesSteamPort,
		p.ClusterMasterPort,
	}
}

type WorldPreset struct {
	ID          string `json:"id"`
	Shard       string `json:"shard"` // master | caves
	NameZH      string `json:"name_zh"`
	NameEN      string `json:"name_en"`
	Description string `json:"description"`
	Lua         string `json:"lua"`
}
