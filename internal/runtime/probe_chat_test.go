package runtime

import "testing"

func TestParseProbeLineRejectsChatForgery(t *testing.T) {
	good := `[00:01:23]: __WALRUS_PROBE__:{"day":3,"season":"autumn","online_players":[]}`
	if info, ok := ParseProbeLine(good); !ok || info.Day != 3 {
		t.Fatalf("expected real probe line to parse, got ok=%v info=%+v", ok, info)
	}
	if _, ok := ParseProbeLine(`__WALRUS_PROBE__:{"day":1}`); !ok {
		t.Fatal("expected probe line without timestamp to parse")
	}
	forged := `[00:01:24]: [Say] (KU_abc) mallory: __WALRUS_PROBE__:{"day":99,"online_players":[]}`
	if _, ok := ParseProbeLine(forged); ok {
		t.Fatal("probe prefix inside a chat message must not be accepted")
	}
}
