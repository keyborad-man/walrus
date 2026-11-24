package dst

import "testing"

func TestWorldSchemaIntegrityAndClientOrder(t *testing.T) {
	cases := []struct {
		shard        string
		wantGroups   []string
		wantOptCount int
	}{
		{
			shard: "master",
			wantGroups: []string{
				"settings_global", "settings_survivors", "settings_misc", "settings_resources", "settings_giants",
				"worldgen_global", "worldgen_misc", "worldgen_resources", "worldgen_animals", "worldgen_monsters",
			},
			wantOptCount: 63,
		},
		{
			shard: "caves",
			wantGroups: []string{
				"settings_misc", "settings_resources", "settings_giants",
				"worldgen_misc", "worldgen_resources", "worldgen_animals", "worldgen_monsters",
			},
			wantOptCount: 25,
		},
	}

	for _, tc := range cases {
		t.Run(tc.shard, func(t *testing.T) {
			groups := WorldSchema(tc.shard)
			if len(groups) != len(tc.wantGroups) {
				t.Fatalf("group count = %d, want %d", len(groups), len(tc.wantGroups))
			}

			seenGroup := map[string]bool{}
			seenKey := map[string]bool{}
			seenWorldgen := false
			total := 0

			for i, g := range groups {
				if g.ID != tc.wantGroups[i] {
					t.Errorf("group[%d].ID = %q, want %q", i, g.ID, tc.wantGroups[i])
				}
				if seenGroup[g.ID] {
					t.Errorf("duplicate group ID %q", g.ID)
				}
				seenGroup[g.ID] = true

				switch g.Section {
				case "settings":
					if seenWorldgen {
						t.Errorf("settings group %q appeared after worldgen", g.ID)
					}
				case "worldgen":
					seenWorldgen = true
				default:
					t.Errorf("group %q has invalid section %q", g.ID, g.Section)
				}

				for j, opt := range g.Options {
					total++
					if seenKey[opt.Key] {
						t.Errorf("duplicate option key %q in %s", opt.Key, tc.shard)
					}
					seenKey[opt.Key] = true

					// Verify default exists in choices and choices are unique
					foundDefault := false
					seenChoice := map[string]bool{}
					for _, ch := range opt.Choices {
						if seenChoice[ch.Value] {
							t.Errorf("option %q has duplicate choice %q", opt.Key, ch.Value)
						}
						seenChoice[ch.Value] = true
						if ch.Value == opt.Default {
							foundDefault = true
						}
					}
					if !foundDefault {
						t.Errorf("option %q default %q not found in choices", opt.Key, opt.Default)
					}

					// Verify DST client item sort rule within group:
					// ordered items (Order > 0) come first sorted ascending by Order;
					// unordered items (Order == 0) follow sorted by LabelZH UTF-8 byte order.
					if j > 0 {
						prev := g.Options[j-1]
						if prev.Order == 0 && opt.Order > 0 {
							t.Errorf("group %q: ordered item %q (%d) appears after unordered item %q", g.ID, opt.Key, opt.Order, prev.Key)
						} else if prev.Order > 0 && opt.Order > 0 && prev.Order > opt.Order {
							t.Errorf("group %q: order not ascending: %q (%d) before %q (%d)", g.ID, prev.Key, prev.Order, opt.Key, opt.Order)
						} else if prev.Order == 0 && opt.Order == 0 && prev.LabelZH > opt.LabelZH {
							t.Errorf("group %q: Chinese client UTF-8 order broken: %q (%s) before %q (%s)", g.ID, prev.Key, prev.LabelZH, opt.Key, opt.LabelZH)
						}
					}
				}
			}

			if total != tc.wantOptCount {
				t.Errorf("total options = %d, want %d", total, tc.wantOptCount)
			}
		})
	}
}

func TestWorldSchemaOfficialFixes(t *testing.T) {
	findOpt := func(shard, key string) *WorldOptionField {
		for _, g := range WorldSchema(shard) {
			for i := range g.Options {
				if g.Options[i].Key == key {
					return &g.Options[i]
				}
			}
		}
		return nil
	}

	if findOpt("master", "lightninggoat") == nil {
		t.Fatal("expected master option key 'lightninggoat'")
	}
	if findOpt("master", "voltgoat") != nil {
		t.Fatal("deprecated master key 'voltgoat' should be replaced by 'lightninggoat'")
	}
	if findOpt("caves", "daywalker") == nil {
		t.Fatal("expected caves option key 'daywalker'")
	}
	if findOpt("caves", "nightmarewerepig") != nil {
		t.Fatal("deprecated caves key 'nightmarewerepig' should be replaced by 'daywalker'")
	}

	for _, tc := range []struct {
		shard, key, wantDef string
	}{
		{"master", "ghostsanitydrain", "always"},
		{"master", "healthpenalty", "always"},
		{"caves", "acidrain_enabled", "always"},
	} {
		opt := findOpt(tc.shard, tc.key)
		if opt == nil || opt.Default != tc.wantDef {
			t.Fatalf("%s/%s default mismatch: got %+v, want %q", tc.shard, tc.key, opt, tc.wantDef)
		}
	}

	pet := findOpt("master", "petrification")
	wantPet := []string{"none", "few", "default", "many", "max"}
	if pet == nil || len(pet.Choices) != len(wantPet) {
		t.Fatalf("petrification choices unexpected: %+v", pet)
	}
	for i, w := range wantPet {
		if pet.Choices[i].Value != w {
			t.Errorf("petrification choice[%d] = %q, want %q", i, pet.Choices[i].Value, w)
		}
	}

	loop := findOpt("master", "loop")
	wantLoop := []string{"never", "default", "always"}
	if loop == nil || len(loop.Choices) != len(wantLoop) {
		t.Fatalf("loop choices unexpected: %+v", loop)
	}
	for i, w := range wantLoop {
		if loop.Choices[i].Value != w {
			t.Errorf("loop choice[%d] = %q, want %q", i, loop.Choices[i].Value, w)
		}
	}
}
