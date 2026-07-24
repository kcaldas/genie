package session

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Level controls how much a Recorder captures.
type Level int

const (
	// LevelOff disables recording entirely (NewRecorder returns nil).
	LevelOff Level = iota
	// LevelStandard captures turns with tight excerpt caps (default hosted level).
	LevelStandard
	// LevelFull captures turns with generous excerpt caps.
	LevelFull
)

// String returns the knob spelling for the level.
func (l Level) String() string {
	switch l {
	case LevelStandard:
		return "standard"
	case LevelFull:
		return "full"
	default:
		return "off"
	}
}

// ParseLevel parses the recording level knob. Empty means off. Unknown
// values return an error alongside LevelOff so callers can warn and keep
// the safe default.
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off":
		return LevelOff, nil
	case "standard":
		return LevelStandard, nil
	case "full":
		return LevelFull, nil
	default:
		return LevelOff, fmt.Errorf("unknown session recording level %q (want off, standard or full)", s)
	}
}

// caps holds the hard limits for a recording level.
type caps struct {
	maxFieldBytes     int
	maxEntriesPerTurn int
}

func capsFor(level Level) caps {
	switch level {
	case LevelFull:
		return caps{maxFieldBytes: 8192, maxEntriesPerTurn: 500}
	default:
		return caps{maxFieldBytes: 1024, maxEntriesPerTurn: 200}
	}
}

// capField builds a Field from text, truncating to max bytes on a valid
// UTF-8 boundary and stamping the truncation marker.
func capField(text string, max int) Field {
	if len(text) <= max {
		return Field{Text: text}
	}
	cut := text[:max]
	for !utf8.ValidString(cut) && len(cut) > 0 {
		cut = cut[:len(cut)-1]
	}
	return Field{Text: cut, Truncated: true, OrigBytes: len(text)}
}
