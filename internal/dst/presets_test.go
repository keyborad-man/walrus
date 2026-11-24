package dst_test

import (
	"testing"

	"walrus/internal/dst"
	"walrus/internal/lua"
)

func TestBuiltinPresetsHaveValidDSTWorldgenFields(t *testing.T) {
	validForestIDs := map[string]bool{
		"SURVIVAL_TOGETHER": true,
		"COMPLETE_DARKNESS": true,
	}
	validCaveIDs := map[string]bool{
		"DST_CAVE": true,
	}

	seenSettingsIDs := map[string]string{}

	for _, preset := range dst.BuiltinPresets {
		parsed, err := lua.ParseLevelData(preset.Lua)
		if err != nil {
			t.Fatalf("preset %q failed to parse: %v", preset.ID, err)
		}

		if parsed.SettingsID == "" {
			t.Errorf("preset %q missing settings_id", preset.ID)
		} else {
			key := preset.Shard + ":" + parsed.SettingsID
			if prev, dup := seenSettingsIDs[key]; dup {
				t.Errorf("preset %q duplicates settings_id %q already used by %q", preset.ID, parsed.SettingsID, prev)
			}
			seenSettingsIDs[key] = preset.ID
		}

		if parsed.ID != parsed.WorldgenID {
			t.Errorf("preset %q has mismatched id=%q and worldgen_id=%q", preset.ID, parsed.ID, parsed.WorldgenID)
		}

		taskSet, _ := parsed.Overrides["task_set"].(string)
		startLoc, _ := parsed.Overrides["start_location"].(string)
		layoutMode, _ := parsed.Overrides["layout_mode"].(string)
		wormholePrefab, _ := parsed.Overrides["wormhole_prefab"].(string)
		roads, _ := parsed.Overrides["roads"].(string)
		seasonStart, _ := parsed.Overrides["season_start"].(string)
		if taskSet == "" {
			t.Errorf("preset %q missing overrides.task_set", preset.ID)
		}
		if startLoc == "" {
			t.Errorf("preset %q missing overrides.start_location", preset.ID)
		}
		if layoutMode == "" {
			t.Errorf("preset %q missing overrides.layout_mode", preset.ID)
		}
		if wormholePrefab == "" {
			t.Errorf("preset %q missing overrides.wormhole_prefab", preset.ID)
		}
		if roads == "" {
			t.Errorf("preset %q missing overrides.roads", preset.ID)
		}
		if seasonStart == "" {
			t.Errorf("preset %q missing overrides.season_start", preset.ID)
		}

		switch preset.Shard {
		case "master":
			if !validForestIDs[parsed.ID] {
				t.Errorf("master preset %q has invalid DST worldgen id %q", preset.ID, parsed.ID)
			}
			if taskSet != "default" {
				t.Errorf("master preset %q expected task_set=default, got %q", preset.ID, taskSet)
			}
			if layoutMode != "LinkNodesByKeys" {
				t.Errorf("master preset %q expected layout_mode=LinkNodesByKeys, got %q", preset.ID, layoutMode)
			}
			if wormholePrefab != "wormhole" {
				t.Errorf("master preset %q expected wormhole_prefab=wormhole, got %q", preset.ID, wormholePrefab)
			}
			if roads != "default" {
				t.Errorf("master preset %q expected roads=default, got %q", preset.ID, roads)
			}
		case "caves":
			if !validCaveIDs[parsed.ID] {
				t.Errorf("caves preset %q has invalid DST worldgen id %q", preset.ID, parsed.ID)
			}
			if taskSet != "cave_default" {
				t.Errorf("caves preset %q expected task_set=cave_default, got %q", preset.ID, taskSet)
			}
			if layoutMode != "RestrictNodesByKey" {
				t.Errorf("caves preset %q expected layout_mode=RestrictNodesByKey, got %q", preset.ID, layoutMode)
			}
			if wormholePrefab != "tentacle_pillar" {
				t.Errorf("caves preset %q expected wormhole_prefab=tentacle_pillar, got %q", preset.ID, wormholePrefab)
			}
			if roads != "never" {
				t.Errorf("caves preset %q expected roads=never, got %q", preset.ID, roads)
			}
		default:
			t.Errorf("preset %q has unexpected shard %q", preset.ID, preset.Shard)
		}

		if _, changed, err := lua.EnsureLevelDataDefaults(preset.Lua, preset.Shard); err != nil {
			t.Errorf("preset %q EnsureLevelDataDefaults error: %v", preset.ID, err)
		} else if changed {
			t.Errorf("preset %q is missing mandatory DST location defaults (EnsureLevelDataDefaults reported changes)", preset.ID)
		}
	}
}
