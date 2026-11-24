package lua

import (
	"strings"
	"testing"
)

func TestModInfoSandboxAndLocale(t *testing.T) {
	sample := `
local is_zh = (locale == "zh" or locale == "zhr")
name = is_zh and "全球定位" or "Global Positions"
description = ChooseTranslationTable({ [1] = "Share map pins", zh = "与队友共享地图位置" })
author = "rezecib"
version = "1.7.4"
api_version = 10
dst_compatible = true
all_clients_require_mod = true
client_only_mod = false
server_filter_tags = { "global positions", "map" }
configuration_options = {
	{
		name = "SHOWPLAYERS",
		label = is_zh and "显示玩家位置" or "Show Players",
		hover = is_zh and "在地图上显示队友" or "Show teammates on map",
		options = {
			{ description = is_zh and "总是" or "Always", data = "always" },
			{ description = is_zh and "默认" or "Default", data = "default" },
			{ description = is_zh and "从不" or "Never", data = "never" },
		},
		default = "default",
	},
	{
		name = "FIREOPTIONS",
		label = is_zh and "信号火" or "Signal Fires",
		options = {
			{ description = "On", data = true },
			{ description = "Off", data = false },
		},
		default = true,
	},
}
`
	zhInfo, err := ParseModInfo(sample, "zh-CN", "workshop-378160973")
	if err != nil {
		t.Fatalf("parse zh modinfo: %v", err)
	}
	if zhInfo.Name != "全球定位" || zhInfo.Description != "与队友共享地图位置" {
		t.Fatalf("expected zh strings, got name=%q desc=%q", zhInfo.Name, zhInfo.Description)
	}
	if len(zhInfo.ConfigurationOptions) != 2 || zhInfo.ConfigurationOptions[0].Label != "显示玩家位置" {
		t.Fatalf("unexpected zh options: %+v", zhInfo.ConfigurationOptions)
	}

	enInfo, err := ParseModInfo(sample, "en-US", "workshop-378160973")
	if err != nil {
		t.Fatalf("parse en modinfo: %v", err)
	}
	if enInfo.Name != "Global Positions" || enInfo.Description != "Share map pins" {
		t.Fatalf("expected en strings, got name=%q desc=%q", enInfo.Name, enInfo.Description)
	}

	// Verify dangerous calls fail in sandbox
	if _, err := ParseModInfo(`os.execute("echo pwn")`, "en-US", "bad"); err == nil {
		t.Fatal("expected os.execute to fail in sandbox")
	}
	if _, err := ParseModInfo(`io.open("/etc/passwd")`, "en-US", "bad"); err == nil {
		t.Fatal("expected io.open to fail in sandbox")
	}
}

func TestLosslessLevelDataPatchAndModOverridesRoundTrip(t *testing.T) {
	orig := `return {
  id = "ENDLESS",
  location = "forest",
  overrides = {
    world_size = "default",
    my_custom_mod_option = "super_rare",
  },
  required_prefabs = { "multiplayer_portal" },
  version = 4,
}`
	patched, err := PatchLevelOverrides(orig, map[string]any{
		"world_size": "huge",
		"autumn":     "longseason",
	}, false)
	if err != nil {
		t.Fatalf("patch level overrides: %v", err)
	}
	if !strings.Contains(patched, `my_custom_mod_option = "super_rare"`) {
		t.Fatalf("expected custom mod override to be preserved:\n%s", patched)
	}
	parsed, err := ParseLevelData(patched)
	if err != nil {
		t.Fatalf("re-parse patched lua: %v", err)
	}
	if parsed.Overrides["world_size"] != "huge" || parsed.Overrides["autumn"] != "longseason" || parsed.Overrides["my_custom_mod_option"] != "super_rare" {
		t.Fatalf("unexpected parsed overrides: %+v", parsed.Overrides)
	}

	// Mod overrides round-trip
	mods := map[string]ModOverride{
		"workshop-378160973": {
			Enabled: true,
			ConfigurationOptions: map[string]any{
				"SHOWPLAYERS": "always",
				"FIREOPTIONS": true,
				"SCALE":       int64(2),
			},
		},
	}
	ser := SerializeModOverrides(mods)
	got, err := ParseModOverrides(ser)
	if err != nil {
		t.Fatalf("parse serialized modoverrides: %v", err)
	}
	if !got["workshop-378160973"].Enabled || got["workshop-378160973"].ConfigurationOptions["SHOWPLAYERS"] != "always" {
		t.Fatalf("modoverrides roundtrip mismatch: %+v", got)
	}
}
