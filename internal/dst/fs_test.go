package dst

import (
	"testing"
)

func TestClusterINIAndPortPool(t *testing.T) {
	root := t.TempDir()
	fs := NewFS(root)

	if err := ValidateClusterID("bad/id"); err == nil {
		t.Fatal("expected invalid cluster id error")
	}
	if err := ValidateClusterID("forest-01"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Allocate ports for first cluster
	p1, err := fs.AllocatePorts(10990, 11020)
	if err != nil {
		t.Fatalf("allocate p1: %v", err)
	}
	if p1.MasterServerPort != 10990 || p1.ClusterMasterPort != 10996 {
		t.Fatalf("unexpected p1 ports: %+v", p1)
	}

	ci := DefaultClusterINI("Friends Survival", p1.ClusterMasterPort, true)
	ci.MaxPlayers = 8
	ci.ClusterPassword = "secret"
	if err := fs.WriteClusterINI("cluster_a", &ci); err != nil {
		t.Fatalf("write cluster.ini: %v", err)
	}
	if err := fs.WriteServerINI("cluster_a", "master", &ServerINI{
		ServerPort: p1.MasterServerPort, IsMaster: true, Name: "Master", ID: "1",
		EncodeUserPath: true, MasterServerPort: p1.MasterSteamPort, AuthenticationPort: p1.MasterAuthPort,
	}); err != nil {
		t.Fatalf("write master server.ini: %v", err)
	}
	if err := fs.WriteServerINI("cluster_a", "caves", &ServerINI{
		ServerPort: p1.CavesServerPort, IsMaster: false, Name: "Caves", ID: "2",
		EncodeUserPath: true, MasterServerPort: p1.CavesSteamPort, AuthenticationPort: p1.CavesAuthPort,
	}); err != nil {
		t.Fatalf("write caves server.ini: %v", err)
	}

	got, err := fs.ReadClusterINI("cluster_a")
	if err != nil {
		t.Fatalf("read cluster.ini: %v", err)
	}
	if got.ClusterName != "Friends Survival" || got.MaxPlayers != 8 || got.ClusterPassword != "secret" || got.MasterPort != p1.ClusterMasterPort {
		t.Fatalf("cluster.ini mismatch: %+v", got)
	}

	// Second cluster must get non-overlapping ports starting at 10997
	p2, err := fs.AllocatePorts(10990, 11020)
	if err != nil {
		t.Fatalf("allocate p2: %v", err)
	}
	if p2.MasterServerPort != 10997 {
		t.Fatalf("expected second cluster master port 10997, got %d", p2.MasterServerPort)
	}

	// Conflict validation must reject overlapping port from cluster_a
	conflict := *p2
	conflict.MasterServerPort = p1.MasterServerPort
	if err := fs.ValidatePortConflicts("cluster_b", conflict, true); err == nil {
		t.Fatal("expected port conflict error, got nil")
	}
}
