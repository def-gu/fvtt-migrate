package progress

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Phase string

const (
	PhaseHashing      Phase = "hashing"
	PhaseNegotiating  Phase = "negotiating"
	PhaseTransferring Phase = "transferring"
	PhasePlacing      Phase = "placing"
	PhaseVerifying    Phase = "verifying"
)

type Event struct {
	Type   string `json:"type"`
	Phase  Phase  `json:"phase"`
	Done   int64  `json:"done"`
	Total  int64  `json:"total"`
	Bytes  int64  `json:"bytes,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type Level string

const (
	LevelNote    Level = "note"
	LevelWarning Level = "warning"
)

// Code lets a caller translate or act on a message without matching English
// prose, which is the point for an interface that will not be in English.
type Notice struct {
	Type    string            `json:"type"`
	Level   Level             `json:"level"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Params  map[string]string `json:"params,omitempty"`
}

// A nil Sink is valid and discards everything, so callers never branch.
type Sink interface {
	Emit(any)
}

type sinkFunc func(any)

func (f sinkFunc) Emit(v any) {
	if f != nil {
		f(v)
	}
}

func Emit(s Sink, v any) {
	if s == nil {
		return
	}
	switch e := v.(type) {
	case Event:
		e.Type = "progress"
		s.Emit(e)
	case Notice:
		e.Type = "notice"
		s.Emit(e)
	default:
		s.Emit(v)
	}
}

func Note(s Sink, code, message string, params map[string]string) {
	Emit(s, Notice{Level: LevelNote, Code: code, Message: message, Params: params})
}

func Warn(s Sink, code, message string, params map[string]string) {
	Emit(s, Notice{Level: LevelWarning, Code: code, Message: message, Params: params})
}

// Throttle drops intermediate progress but never the last event of a phase, so
// a consumer always learns that a phase completed. Notices are never dropped.
func Throttle(inner Sink, every time.Duration) Sink {
	var mu sync.Mutex
	var last time.Time
	var lastPhase Phase

	return sinkFunc(func(v any) {
		e, ok := v.(Event)
		if !ok {
			inner.Emit(v)
			return
		}

		mu.Lock()
		final := e.Total > 0 && e.Done >= e.Total
		skip := !final && e.Phase == lastPhase && time.Since(last) < every
		if !skip {
			last, lastPhase = time.Now(), e.Phase
		}
		mu.Unlock()

		if !skip {
			inner.Emit(e)
		}
	})
}

func Lines(w io.Writer) Sink {
	var mu sync.Mutex
	enc := json.NewEncoder(w)
	return Throttle(sinkFunc(func(v any) {
		mu.Lock()
		defer mu.Unlock()
		enc.Encode(v)
	}), 100*time.Millisecond)
}

func Ticker(w io.Writer) Sink {
	var mu sync.Mutex
	var pending bool

	// The clear-line escape is only correct once a progress line is on screen;
	// emitting it otherwise leaks into whatever else is being printed.
	clear := func() string {
		if pending {
			pending = false
			return "\r\033[K"
		}
		return ""
	}

	return Throttle(sinkFunc(func(v any) {
		mu.Lock()
		defer mu.Unlock()

		if n, ok := v.(Notice); ok {
			fmt.Fprintf(w, "%s%s: %s\n", clear(), n.Level, n.Message)
			return
		}

		e, ok := v.(Event)
		if !ok {
			return
		}
		line := fmt.Sprintf("%-13s %d", e.Phase, e.Done)
		if e.Total > 0 {
			line += fmt.Sprintf("/%d", e.Total)
		}
		if e.Bytes > 0 {
			line += "  " + Bytes(e.Bytes)
		}
		fmt.Fprintf(w, "%s\r%s", clear(), line)
		pending = true
		if e.Total > 0 && e.Done >= e.Total {
			fmt.Fprintln(w)
			pending = false
		}
	}), 100*time.Millisecond)
}

func Bytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Plain writes one line per event with no escape sequences, for output that is
// not a terminal.
func Plain(w io.Writer) Sink {
	var mu sync.Mutex
	return Throttle(sinkFunc(func(v any) {
		mu.Lock()
		defer mu.Unlock()

		switch e := v.(type) {
		case Notice:
			fmt.Fprintf(w, "%s: %s\n", e.Level, e.Message)
		case Event:
			if e.Total > 0 && e.Done >= e.Total {
				fmt.Fprintf(w, "%s %d/%d\n", e.Phase, e.Done, e.Total)
			}
		}
	}), time.Second)
}

func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
