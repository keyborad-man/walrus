package runtime

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	ProbePrefix   = "__WALRUS_PROBE__:"
	maxRingBuffer = 600
)

type shardLogStream struct {
	mu      sync.RWMutex
	entries []LogEntry
	subs    map[uint64]chan LogEntry
}

type LogBus struct {
	mu      sync.RWMutex
	streams map[string]*shardLogStream
	seq     atomic.Int64
	subSeq  atomic.Uint64
}

func NewLogBus() *LogBus {
	return &LogBus{
		streams: make(map[string]*shardLogStream),
	}
}

func streamKey(clusterID, shard string) string {
	return clusterID + "/" + strings.ToLower(shard)
}

func (b *LogBus) getOrCreate(clusterID, shard string) *shardLogStream {
	k := streamKey(clusterID, shard)
	b.mu.RLock()
	st, ok := b.streams[k]
	b.mu.RUnlock()
	if ok {
		return st
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if st, ok = b.streams[k]; ok {
		return st
	}
	st = &shardLogStream{
		entries: make([]LogEntry, 0, maxRingBuffer),
		subs:    make(map[uint64]chan LogEntry),
	}
	b.streams[k] = st
	return st
}

// Publish appends a log line to the shard buffer and broadcasts to live subscribers.
// Returns false if the line was suppressed (e.g., internal silent probe output).
func (b *LogBus) Publish(clusterID, shard, level, message string) bool {
	if strings.Contains(message, ProbePrefix) {
		return false
	}
	if level == "" {
		level = ClassifyLogLevel(message)
	}
	entry := LogEntry{
		Seq:       b.seq.Add(1),
		Timestamp: time.Now().UTC(),
		Shard:     strings.ToLower(shard),
		Level:     level,
		Message:   message,
	}
	st := b.getOrCreate(clusterID, shard)
	st.mu.Lock()
	if len(st.entries) >= maxRingBuffer {
		copy(st.entries, st.entries[1:])
		st.entries[len(st.entries)-1] = entry
	} else {
		st.entries = append(st.entries, entry)
	}
	for _, ch := range st.subs {
		select {
		case ch <- entry:
		default:
			// Drop oldest behavior if slow consumer; skip to keep server non-blocking.
		}
	}
	st.mu.Unlock()
	return true
}

func (b *LogBus) Recent(clusterID, shard string, limit int) []LogEntry {
	if limit <= 0 || limit > maxRingBuffer {
		limit = 200
	}
	st := b.getOrCreate(clusterID, shard)
	st.mu.RLock()
	defer st.mu.RUnlock()
	n := len(st.entries)
	if n == 0 {
		return nil
	}
	start := 0
	if n > limit {
		start = n - limit
	}
	out := make([]LogEntry, n-start)
	copy(out, st.entries[start:])
	return out
}

func (b *LogBus) Subscribe(clusterID, shard string) (<-chan LogEntry, func()) {
	st := b.getOrCreate(clusterID, shard)
	ch := make(chan LogEntry, 128)
	id := b.subSeq.Add(1)
	st.mu.Lock()
	st.subs[id] = ch
	st.mu.Unlock()

	cancel := func() {
		st.mu.Lock()
		if _, ok := st.subs[id]; ok {
			delete(st.subs, id)
			close(ch)
		}
		st.mu.Unlock()
	}
	return ch, cancel
}

func ClassifyLogLevel(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "lua error") || strings.Contains(lower, "crash"):
		return "error"
	case strings.Contains(lower, "warning") || strings.Contains(lower, "warn:"):
		return "warn"
	case strings.Contains(msg, "[Say]") || strings.Contains(msg, "[Announcement]") || strings.Contains(msg, "[Join Announcement]") || strings.Contains(msg, "[Leave Announcement]"):
		return "chat"
	case strings.HasPrefix(msg, "[walrus]"):
		return "system"
	default:
		return "info"
	}
}

// ReadTailFromFile reads the last `limit` lines from server_log.txt as fallback when memory buffer is empty.
func ReadTailFromFile(path, shard string, limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > maxRingBuffer {
		limit = 200
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogEntry{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, ProbePrefix) {
			continue
		}
		lines = append(lines, line)
		if len(lines) > limit*2 {
			lines = lines[len(lines)-limit:]
		}
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	out := make([]LogEntry, 0, len(lines))
	now := time.Now().UTC()
	for i, line := range lines {
		out = append(out, LogEntry{
			Seq:       int64(i + 1),
			Timestamp: now,
			Shard:     strings.ToLower(shard),
			Level:     ClassifyLogLevel(line),
			Message:   line,
		})
	}
	return out, nil
}
