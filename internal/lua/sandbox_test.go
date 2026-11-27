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

func TestParseModInfoSkipsDecorativeDividers(t *testing.T) {
	// Matches Legion (workshop-2007975851) pattern with repeated "Title" section dividers.
	sample := `
name = "[DST] Legion-棱镜"
author = "ti_Tout"
version = "6.1.1"
description = "Version " .. version
api_version = 10
dst_compatible = true
all_clients_require_mod = true
client_only_mod = false
configuration_options = {
	{name = "Title", label = "Language", options = {{description = "", data = ""}}, default = ""},
	{
		name = "Language",
		label = "Language",
		options = {
			{description = "Auto", data = "auto"},
			{description = "中文", data = "chinese"},
		},
		default = "auto",
	},
	{name = "Title", label = "", options = {{description = "", data = ""}}, default = ""},
	{name = "Title", label = "ThePowerOfFlowers", options = {{description = "", data = ""}}, default = ""},
	{
		name = "FlowersPower",
		label = "Flowers' Power",
		options = {
			{description = "Yes", data = true},
			{description = "No", data = false},
		},
		default = true,
	},
}
`
	info, err := ParseModInfo(sample, "zh-CN", "workshop-2007975851")
	if err != nil {
		t.Fatalf("parse legion-style modinfo: %v", err)
	}
	if info.Version != "6.1.1" {
		t.Fatalf("expected version 6.1.1, got %q", info.Version)
	}
	if len(info.ConfigurationOptions) != 2 {
		t.Fatalf("expected 2 real options (skipping Title dividers), got %d: %+v", len(info.ConfigurationOptions), info.ConfigurationOptions)
	}
	if info.ConfigurationOptions[0].Name != "Language" || info.ConfigurationOptions[1].Name != "FlowersPower" {
		t.Fatalf("unexpected option names: %+v", info.ConfigurationOptions)
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

func TestEnsureLevelDataDefaults(t *testing.T) {
	// Legacy forest config missing layout_mode, task_set, wormhole_prefab, etc. (without random_set_pieces)
	legacyForest := `return {
  desc="A never-ending Don't Starve sandbox.",
  id="ENDLESS",
  location="forest",
  name="Endless Forest",
  numrandom_set_pieces=4,
  overrides={
    has_ocean=false,
    portalresurection="always",
    my_mod_opt="custom",
  },
  version=4,
}`
	patched, changed, err := EnsureLevelDataDefaults(legacyForest, "master")
	if err != nil {
		t.Fatalf("EnsureLevelDataDefaults forest: %v", err)
	}
	if !changed {
		t.Fatal("expected legacy forest config to be patched")
	}
	parsed, err := ParseLevelData(patched)
	if err != nil {
		t.Fatalf("re-parse patched forest: %v", err)
	}
	if parsed.Overrides["layout_mode"] != "LinkNodesByKeys" {
		t.Fatalf("expected layout_mode=LinkNodesByKeys, got %v", parsed.Overrides["layout_mode"])
	}
	if parsed.Overrides["task_set"] != "default" {
		t.Fatalf("expected task_set=default, got %v", parsed.Overrides["task_set"])
	}
	if parsed.Overrides["wormhole_prefab"] != "wormhole" {
		t.Fatalf("expected wormhole_prefab=wormhole, got %v", parsed.Overrides["wormhole_prefab"])
	}
	// Explicit false and custom mod keys must never be overwritten.
	if parsed.Overrides["has_ocean"] != false {
		t.Fatalf("expected explicit has_ocean=false to be preserved, got %v", parsed.Overrides["has_ocean"])
	}
	if parsed.Overrides["my_mod_opt"] != "custom" {
		t.Fatalf("expected my_mod_opt=custom to be preserved, got %v", parsed.Overrides["my_mod_opt"])
	}
	// Must NOT add required_setpieces when random_set_pieces is missing (otherwise DST level.lua crashes on #self.random_set_pieces).
	if strings.Contains(patched, "required_setpieces") {
		t.Fatalf("did not expect required_setpieces without random_set_pieces:\n%s", patched)
	}

	// Second pass must be a no-op.
	if _, changed2, err := EnsureLevelDataDefaults(patched, "master"); err != nil || changed2 {
		t.Fatalf("expected idempotent second pass, got changed=%v err=%v", changed2, err)
	}

	// Legacy caves config missing task_set, start_location, layout_mode, roads, wormhole_prefab
	legacyCave := `return {
  id="DST_CAVE",
  location="cave",
  name="The Caves",
  desc="Delve into the caves... together!",
  overrides={
    world_size="default",
  },
  version=4,
}`
	patchedCave, changedCave, err := EnsureLevelDataDefaults(legacyCave, "caves")
	if err != nil {
		t.Fatalf("EnsureLevelDataDefaults cave: %v", err)
	}
	if !changedCave {
		t.Fatal("expected legacy cave config to be patched")
	}
	parsedCave, err := ParseLevelData(patchedCave)
	if err != nil {
		t.Fatalf("re-parse patched cave: %v", err)
	}
	if parsedCave.Overrides["task_set"] != "cave_default" ||
		parsedCave.Overrides["start_location"] != "caves" ||
		parsedCave.Overrides["layout_mode"] != "RestrictNodesByKey" ||
		parsedCave.Overrides["roads"] != "never" ||
		parsedCave.Overrides["wormhole_prefab"] != "tentacle_pillar" {
		t.Fatalf("unexpected patched cave overrides: %+v", parsedCave.Overrides)
	}

	// Custom modded location must be left untouched.
	modLocation := `return { id="SHIPWRECKED", location="shipwrecked", name="SW", desc="SW", overrides={} }`
	if out, changedMod, err := EnsureLevelDataDefaults(modLocation, "master"); err != nil || changedMod || out != modLocation {
		t.Fatalf("expected modded location untouched, got changed=%v err=%v", changedMod, err)
	}
}

func TestSandboxStringRepIsCapped(t *testing.T) {
	if _, err := ParseModInfo(`name = string.rep("a", 4e9)`, "en", "x"); err == nil {
		t.Fatal("expected oversized string.rep to fail")
	}
	info, err := ParseModInfo(`name = string.rep("ab", 3, "-")`, "en", "x")
	if err != nil {
		t.Fatalf("small string.rep should work: %v", err)
	}
	if info.Name != "ab-ab-ab" {
		t.Fatalf("unexpected rep result %q", info.Name)
	}
}

func TestQuoteStringEscapesConsoleInjection(t *testing.T) {
	got := QuoteString("x\"); c_shutdown(false) --\nC:\\")
	want := `"x\"); c_shutdown(false) --\nC:\\"`
	if got != want {
		t.Fatalf("QuoteString = %s, want %s", got, want)
	}
}
