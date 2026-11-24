package lua

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	glua "github.com/yuin/gopher-lua"

	"walrus/internal/apperr"
)

const sandboxTimeout = 2 * time.Second

// ModOptionChoice represents one selectable choice inside a mod configuration option.
type ModOptionChoice struct {
	Description string `json:"description"`
	Data        any    `json:"data"`
	Hover       string `json:"hover,omitempty"`
}

// ModConfigOption represents one item in modinfo.lua's configuration_options list.
type ModConfigOption struct {
	Name    string            `json:"name"`
	Label   string            `json:"label"`
	Hover   string            `json:"hover,omitempty"`
	Default any               `json:"default"`
	Options []ModOptionChoice `json:"options"`
}

// ModInfo is the structured metadata extracted from a DST modinfo.lua file.
type ModInfo struct {
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	Author               string            `json:"author"`
	Version              string            `json:"version"`
	APIVersion           int               `json:"api_version"`
	DSTCompatible        bool              `json:"dst_compatible"`
	AllClientsRequireMod bool              `json:"all_clients_require_mod"`
	ClientOnlyMod        bool              `json:"client_only_mod"`
	ServerFilterTags     []string          `json:"server_filter_tags"`
	ConfigurationOptions []ModConfigOption `json:"configuration_options"`
}

// ModOverride models one entry inside modoverrides.lua.
type ModOverride struct {
	Enabled              bool           `json:"enabled"`
	ConfigurationOptions map[string]any `json:"configuration_options"`
}

// LuaArray preserves Lua sequential arrays during lossless round-trip serialization.
type LuaArray []any

func newSandboxedState(ctx context.Context) *glua.LState {
	L := glua.NewState(glua.Options{
		SkipOpenLibs:        true,
		CallStackSize:       64,
		RegistrySize:        1024,
		IncludeGoStackTrace: false,
	})
	L.SetContext(ctx)

	for _, pair := range []struct {
		name string
		fn   glua.LGFunction
	}{
		{glua.LoadLibName, glua.OpenPackage}, // removed immediately if not needed -> skip package!
	} {
		_ = pair
	}

	// Open strictly safe standard libraries only (no os, io, package, debug).
	glua.OpenBase(L)
	glua.OpenTable(L)
	glua.OpenString(L)
	glua.OpenMath(L)

	// Strip file/code loading builtins from base library.
	for _, banned := range []string{"dofile", "loadfile", "load", "loadstring", "module", "require", "collectgarbage"} {
		L.SetGlobal(banned, glua.LNil)
	}
	return L
}

// NormalizeDSTLocale converts BCP-47 UI locales to DST's modinfo locale convention.
func NormalizeDSTLocale(uiLocale string) string {
	l := strings.ToLower(strings.TrimSpace(uiLocale))
	if strings.HasPrefix(l, "zh") {
		return "zh"
	}
	return "en"
}

// ParseModInfo executes modinfo.lua inside a strict Lua sandbox and extracts metadata + options.
func ParseModInfo(src, uiLocale, folderName string) (*ModInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sandboxTimeout)
	defer cancel()

	L := newSandboxedState(ctx)
	defer L.Close()

	dstLocale := NormalizeDSTLocale(uiLocale)
	L.SetGlobal("locale", glua.LString(dstLocale))
	L.SetGlobal("folder_name", glua.LString(folderName))

	// Provide ChooseTranslationTable(tbl) helper commonly used by DST mods.
	L.SetGlobal("ChooseTranslationTable", L.NewFunction(func(L *glua.LState) int {
		tbl := L.CheckTable(1)
		candidates := []string{dstLocale}
		if dstLocale == "zh" {
			candidates = append(candidates, "chs", "zhr", "zh_cn", "zh_CN", "zht")
		} else {
			candidates = append(candidates, "en", "en_US")
		}
		for _, k := range candidates {
			if v := tbl.RawGetString(k); v != glua.LNil {
				L.Push(v)
				return 1
			}
		}
		if v := tbl.RawGetInt(1); v != glua.LNil {
			L.Push(v)
			return 1
		}
		L.Push(glua.LString(""))
		return 1
	}))

	if err := L.DoString(src); err != nil {
		return nil, apperr.WrapDetail(apperr.CodeModInfoParseFailed, err, "execute modinfo.lua: %v", err)
	}

	info := &ModInfo{
		Name:                 lvString(L.GetGlobal("name")),
		Description:          lvString(L.GetGlobal("description")),
		Author:               lvString(L.GetGlobal("author")),
		Version:              lvString(L.GetGlobal("version")),
		APIVersion:           lvInt(L.GetGlobal("api_version")),
		DSTCompatible:        lvBool(L.GetGlobal("dst_compatible"), true),
		AllClientsRequireMod: lvBool(L.GetGlobal("all_clients_require_mod"), false),
		ClientOnlyMod:        lvBool(L.GetGlobal("client_only_mod"), false),
		ServerFilterTags:     []string{},
		ConfigurationOptions: []ModConfigOption{},
	}

	if tagsTbl, ok := L.GetGlobal("server_filter_tags").(*glua.LTable); ok {
		tagsTbl.ForEach(func(_, v glua.LValue) {
			if s := lvString(v); s != "" {
				info.ServerFilterTags = append(info.ServerFilterTags, s)
			}
		})
	}

	if optsTbl, ok := L.GetGlobal("configuration_options").(*glua.LTable); ok {
		n := optsTbl.MaxN()
		for i := 1; i <= n; i++ {
			item, ok := optsTbl.RawGetInt(i).(*glua.LTable)
			if !ok {
				continue
			}
			opt := ModConfigOption{
				Name:    lvString(item.RawGetString("name")),
				Label:   lvString(item.RawGetString("label")),
				Hover:   lvString(item.RawGetString("hover")),
				Default: luaValueToGo(item.RawGetString("default")),
				Options: []ModOptionChoice{},
			}
			if opt.Label == "" {
				opt.Label = opt.Name
			}
			if choicesTbl, ok := item.RawGetString("options").(*glua.LTable); ok {
				cn := choicesTbl.MaxN()
				for j := 1; j <= cn; j++ {
					ch, ok := choicesTbl.RawGetInt(j).(*glua.LTable)
					if !ok {
						continue
					}
					choice := ModOptionChoice{
						Description: lvString(ch.RawGetString("description")),
						Data:        luaValueToGo(ch.RawGetString("data")),
						Hover:       lvString(ch.RawGetString("hover")),
					}
					opt.Options = append(opt.Options, choice)
				}
			}
			// Keep valid configurable options (skip pure decorative empty dividers with empty name).
			if opt.Name != "" && len(opt.Options) > 0 {
				info.ConfigurationOptions = append(info.ConfigurationOptions, opt)
			}
		}
	}

	return info, nil
}

// --- modoverrides.lua ---

func ParseModOverrides(src string) (map[string]ModOverride, error) {
	if strings.TrimSpace(src) == "" {
		return map[string]ModOverride{}, nil
	}
	val, err := evalReturnTable(src)
	if err != nil {
		return nil, apperr.WrapDetail(apperr.CodeModOverridesParseFail, err, "parse modoverrides.lua: %v", err)
	}
	root, ok := val.(map[string]any)
	if !ok {
		return map[string]ModOverride{}, nil
	}
	res := make(map[string]ModOverride, len(root))
	for modID, rawEntry := range root {
		entryMap, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		ov := ModOverride{
			Enabled:              true,
			ConfigurationOptions: map[string]any{},
		}
		if en, exists := entryMap["enabled"]; exists {
			if b, ok := en.(bool); ok {
				ov.Enabled = b
			}
		}
		if opts, ok := entryMap["configuration_options"].(map[string]any); ok {
			ov.ConfigurationOptions = opts
		}
		res[modID] = ov
	}
	return res, nil
}

func SerializeModOverrides(overrides map[string]ModOverride) string {
	if len(overrides) == 0 {
		return "return {\n}\n"
	}
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("return {\n")
	for _, modID := range keys {
		ov := overrides[modID]
		fmt.Fprintf(&sb, "  [%s] = {\n", quoteLuaString(modID))
		fmt.Fprintf(&sb, "    enabled = %t,\n", ov.Enabled)
		if len(ov.ConfigurationOptions) == 0 {
			sb.WriteString("    configuration_options = {},\n")
		} else {
			sb.WriteString("    configuration_options = ")
			writeLuaValue(&sb, ov.ConfigurationOptions, 2)
			sb.WriteString(",\n")
		}
		sb.WriteString("  },\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

// --- leveldataoverride.lua ---

type ParsedLevelData struct {
	ID         string         `json:"id"`
	SettingsID string         `json:"settings_id"`
	WorldgenID string         `json:"worldgen_id"`
	Name       string         `json:"name"`
	Location   string         `json:"location"`
	Playstyle  string         `json:"playstyle"`
	Overrides  map[string]any `json:"overrides"`
}

func ValidateLevelLua(src string) error {
	val, err := evalReturnTable(src)
	if err != nil {
		return err
	}
	if _, ok := val.(map[string]any); !ok {
		return apperr.New(apperr.CodeLevelDataMustBeTable, "leveldataoverride.lua must return a Lua table")
	}
	return nil
}

func ParseLevelData(src string) (*ParsedLevelData, error) {
	val, err := evalReturnTable(src)
	if err != nil {
		return nil, apperr.WrapDetail(apperr.CodeLuaParseFailed, err, "parse leveldataoverride.lua: %v", err)
	}
	root, ok := val.(map[string]any)
	if !ok {
		return nil, apperr.New(apperr.CodeLevelDataMustBeTable, "leveldataoverride.lua must return a Lua table")
	}
	out := &ParsedLevelData{
		Overrides: map[string]any{},
	}
	if s, ok := root["id"].(string); ok {
		out.ID = s
	}
	if s, ok := root["settings_id"].(string); ok {
		out.SettingsID = s
	}
	if s, ok := root["worldgen_id"].(string); ok {
		out.WorldgenID = s
	}
	if s, ok := root["name"].(string); ok {
		out.Name = s
	}
	if s, ok := root["location"].(string); ok {
		out.Location = s
	}
	if s, ok := root["playstyle"].(string); ok {
		out.Playstyle = s
	}
	if ov, ok := root["overrides"].(map[string]any); ok {
		out.Overrides = ov
	}
	return out, nil
}

// PatchLevelOverrides updates only the specified override keys while preserving all top-level fields and unknown/Mod overrides.
func PatchLevelOverrides(src string, overrides map[string]any, replaceAllOverrides bool) (string, error) {
	val, err := evalReturnTable(src)
	if err != nil {
		return "", apperr.WrapDetail(apperr.CodeLuaParseFailed, err, "parse existing leveldataoverride.lua: %v", err)
	}
	root, ok := val.(map[string]any)
	if !ok {
		return "", apperr.New(apperr.CodeLevelDataMustBeTable, "existing leveldataoverride.lua is not a table")
	}

	existingOv, _ := root["overrides"].(map[string]any)
	if existingOv == nil || replaceAllOverrides {
		existingOv = map[string]any{}
	}
	for k, v := range overrides {
		if v == nil {
			delete(existingOv, k)
		} else {
			existingOv[k] = v
		}
	}
	root["overrides"] = existingOv

	var sb strings.Builder
	sb.WriteString("return ")
	writeLuaValue(&sb, root, 0)
	sb.WriteString("\n")
	return sb.String(), nil
}

// --- Internal Lua <-> Go Conversion & Serialization ---

func evalReturnTable(src string) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sandboxTimeout)
	defer cancel()

	L := newSandboxedState(ctx)
	defer L.Close()

	if err := L.DoString(src); err != nil {
		return nil, err
	}
	if L.GetTop() < 1 {
		return map[string]any{}, nil
	}
	return luaValueToGo(L.Get(-1)), nil
}

func luaValueToGo(lv glua.LValue) any {
	switch v := lv.(type) {
	case *glua.LNilType:
		return nil
	case glua.LBool:
		return bool(v)
	case glua.LNumber:
		f := float64(v)
		if math.Trunc(f) == f && f >= math.MinInt64 && f <= math.MaxInt64 {
			return int64(f)
		}
		return f
	case glua.LString:
		return string(v)
	case *glua.LTable:
		return luaTableToGo(v)
	default:
		return lv.String()
	}
}

func luaTableToGo(tbl *glua.LTable) any {
	maxN := tbl.MaxN()
	totalKeys := 0
	allSequentialInts := true
	tbl.ForEach(func(k, _ glua.LValue) {
		totalKeys++
		if num, ok := k.(glua.LNumber); !ok {
			allSequentialInts = false
		} else {
			idx := int(num)
			if float64(idx) != float64(num) || idx < 1 || idx > maxN {
				allSequentialInts = false
			}
		}
	})

	if totalKeys == 0 {
		return map[string]any{}
	}
	if allSequentialInts && maxN == totalKeys {
		arr := make(LuaArray, 0, maxN)
		for i := 1; i <= maxN; i++ {
			arr = append(arr, luaValueToGo(tbl.RawGetInt(i)))
		}
		return arr
	}

	m := make(map[string]any, totalKeys)
	tbl.ForEach(func(k, v glua.LValue) {
		m[k.String()] = luaValueToGo(v)
	})
	return m
}

func writeLuaValue(sb *strings.Builder, val any, indent int) {
	pad := strings.Repeat("  ", indent)
	nextPad := strings.Repeat("  ", indent+1)

	switch v := val.(type) {
	case nil:
		sb.WriteString("nil")
	case bool:
		if v {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case int:
		sb.WriteString(strconv.Itoa(v))
	case int64:
		sb.WriteString(strconv.FormatInt(v, 10))
	case float64:
		if math.Trunc(v) == v && math.Abs(v) < 1e15 {
			sb.WriteString(strconv.FormatInt(int64(v), 10))
		} else {
			sb.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		}
	case string:
		sb.WriteString(quoteLuaString(v))
	case LuaArray:
		if len(v) == 0 {
			sb.WriteString("{}")
			return
		}
		sb.WriteString("{\n")
		for _, elem := range v {
			sb.WriteString(nextPad)
			writeLuaValue(sb, elem, indent+1)
			sb.WriteString(",\n")
		}
		sb.WriteString(pad)
		sb.WriteString("}")
	case []any:
		writeLuaValue(sb, LuaArray(v), indent)
	case map[string]any:
		if len(v) == 0 {
			sb.WriteString("{}")
			return
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sb.WriteString("{\n")
		for _, k := range keys {
			sb.WriteString(nextPad)
			if isLuaIdent(k) {
				sb.WriteString(k)
				sb.WriteString(" = ")
			} else {
				sb.WriteString("[")
				sb.WriteString(quoteLuaString(k))
				sb.WriteString("] = ")
			}
			writeLuaValue(sb, v[k], indent+1)
			sb.WriteString(",\n")
		}
		sb.WriteString(pad)
		sb.WriteString("}")
	default:
		sb.WriteString(quoteLuaString(fmt.Sprintf("%v", v)))
	}
}

func quoteLuaString(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

var luaReserved = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true,
	"end": true, "false": true, "for": true, "function": true, "if": true,
	"in": true, "local": true, "nil": true, "not": true, "or": true,
	"repeat": true, "return": true, "then": true, "true": true, "until": true, "while": true,
}

func isLuaIdent(s string) bool {
	if s == "" || luaReserved[s] {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
	}
	return true
}

func lvString(v glua.LValue) string {
	if v == glua.LNil {
		return ""
	}
	return v.String()
}

func lvInt(v glua.LValue) int {
	if n, ok := v.(glua.LNumber); ok {
		return int(n)
	}
	return 0
}

func lvBool(v glua.LValue, def bool) bool {
	if v == glua.LNil {
		return def
	}
	if b, ok := v.(glua.LBool); ok {
		return bool(b)
	}
	return def
}
