package runtime

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// SilentProbeLua is a side-effect-free single-line Lua command executed on Master
// to emit structured JSON state prefixed with ProbePrefix (automatically filtered from console UI).
const SilentProbeLua = `local s=TheWorld and TheWorld.state; local ps={}; for _,p in ipairs(AllPlayers or {}) do table.insert(ps,string.format('{"ku_id":%q,"name":%q,"prefab":%q,"age":%d}',tostring(p.userid or ""),tostring(p.name or ""),tostring(p.prefab or ""),math.floor((p.components and p.components.age and p.components.age:GetDisplayAgeInDays()) or 0))) end; print("__WALRUS_PROBE__:"..string.format('{"day":%d,"season":%q,"remaining_days_in_season":%d,"phase":%q,"online_players":[%s]}',s and ((s.cycles or 0)+1) or 0,s and tostring(s.season or "unknown") or "unknown",s and math.floor(s.remainingdaysinseason or 0) or 0,s and tostring(s.phase or "unknown") or "unknown",table.concat(ps,",")))`

// ParseProbeLine parses a line that starts with ProbePrefix (after the optional
// "[hh:mm:ss]: " log timestamp) into WorldRuntimeInfo. The prefix must lead the
// message so players cannot forge probe output by typing it into chat.
func ParseProbeLine(line string) (WorldRuntimeInfo, bool) {
	body := strings.TrimSpace(line)
	if m := tsPrefixRe.FindStringSubmatch(body); len(m) == 3 {
		body = strings.TrimSpace(m[2])
	}
	if !strings.HasPrefix(body, ProbePrefix) {
		return WorldRuntimeInfo{}, false
	}
	rawJSON := strings.TrimSpace(body[len(ProbePrefix):])
	var info WorldRuntimeInfo
	if err := json.Unmarshal([]byte(rawJSON), &info); err != nil {
		return WorldRuntimeInfo{}, false
	}
	if info.OnlinePlayers == nil {
		info.OnlinePlayers = []OnlinePlayer{}
	}
	return info, true
}

var (
	tsPrefixRe = regexp.MustCompile(`^\[([^\]]+)\]:\s*(.*)$`)
	sayRe      = regexp.MustCompile(`^\[Say\]\s*\(([^)]+)\)\s*([^:]+):\s*(.*)$`)
	whisperRe  = regexp.MustCompile(`^\[Whisper\]\s*\(([^)]+)\)\s*([^:]+):\s*(.*)$`)
)

// ParseChatLine parses a DST server_chat_log.txt or chat-related server_log.txt line.
func ParseChatLine(line string) (ChatMessage, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return ChatMessage{}, false
	}
	ts := ""
	body := line
	if m := tsPrefixRe.FindStringSubmatch(line); len(m) == 3 {
		ts = m[1]
		body = strings.TrimSpace(m[2])
	}

	if m := sayRe.FindStringSubmatch(body); len(m) == 4 {
		return ChatMessage{Timestamp: ts, Kind: "say", KUID: strings.TrimSpace(m[1]), Sender: strings.TrimSpace(m[2]), Message: m[3]}, true
	}
	if m := whisperRe.FindStringSubmatch(body); len(m) == 4 {
		return ChatMessage{Timestamp: ts, Kind: "say", KUID: strings.TrimSpace(m[1]), Sender: strings.TrimSpace(m[2]), Message: m[3]}, true
	}
	if strings.HasPrefix(body, "[Announcement]") {
		return ChatMessage{Timestamp: ts, Kind: "announcement", Sender: "System", Message: strings.TrimSpace(strings.TrimPrefix(body, "[Announcement]"))}, true
	}
	if strings.HasPrefix(body, "[Join Announcement]") {
		return ChatMessage{Timestamp: ts, Kind: "join", Sender: strings.TrimSpace(strings.TrimPrefix(body, "[Join Announcement]")), Message: body}, true
	}
	if strings.HasPrefix(body, "[Leave Announcement]") {
		return ChatMessage{Timestamp: ts, Kind: "leave", Sender: strings.TrimSpace(strings.TrimPrefix(body, "[Leave Announcement]")), Message: body}, true
	}
	if strings.HasPrefix(body, "[Death Announcement]") || strings.HasPrefix(body, "[Resurrect Announcement]") {
		return ChatMessage{Timestamp: ts, Kind: "death", Message: body}, true
	}
	return ChatMessage{}, false
}

// ReadChatLogFile reads the most recent chat messages from Master/server_chat_log.txt (falling back to server_log.txt).
func ReadChatLogFile(chatPath, serverLogPath string, limit int) ([]ChatMessage, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	target := chatPath
	if _, err := os.Stat(target); err != nil {
		target = serverLogPath
	}
	f, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			return []ChatMessage{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var msgs []ChatMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for sc.Scan() {
		if msg, ok := ParseChatLine(sc.Text()); ok {
			msgs = append(msgs, msg)
			if len(msgs) > limit*2 {
				msgs = msgs[len(msgs)-limit:]
			}
		}
	}
	if len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	if msgs == nil {
		return []ChatMessage{}, nil
	}
	return msgs, nil
}
