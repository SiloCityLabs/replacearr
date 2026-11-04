package applog

import (
	"sync"
	"time"
)

type Entry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Logger  string    `json:"logger,omitempty"`
	Message string    `json:"message"`
}

const maxEntries = 2000

var (
	mu      sync.RWMutex
	ring    = make([]Entry, 0, maxEntries)
	minLevel = "info"
)

func SetMinLevel(level string) {
	mu.Lock()
	defer mu.Unlock()
	if level == "" {
		level = "info"
	}
	minLevel = level
}

func Info(logger, msg string)  { log("info", logger, msg) }
func Warn(logger, msg string)  { log("warn", logger, msg) }
func Error(logger, msg string) { log("error", logger, msg) }
func Debug(logger, msg string) { log("debug", logger, msg) }

func log(level, logger, msg string) {
	mu.Lock()
	defer mu.Unlock()
	if !levelAllowed(level, minLevel) {
		return
	}
	ring = append(ring, Entry{
		Time:    time.Now().UTC(),
		Level:   level,
		Logger:  logger,
		Message: msg,
	})
	if len(ring) > maxEntries {
		ring = ring[len(ring)-maxEntries:]
	}
}

func List(level string, limit int) []Entry {
	mu.RLock()
	defer mu.RUnlock()
	if limit <= 0 || limit > maxEntries {
		limit = 200
	}
	out := make([]Entry, 0, limit)
	for i := len(ring) - 1; i >= 0 && len(out) < limit; i-- {
		e := ring[i]
		if level != "" && e.Level != level {
			continue
		}
		out = append(out, e)
	}
	return out
}

func levelAllowed(level, min string) bool {
	rank := map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3}
	return rank[level] >= rank[min]
}
