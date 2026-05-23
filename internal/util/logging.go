package util

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Production-grade leveled logger
// ---------------------------------------------------------------------------

var (
	startOnce sync.Once
	startTime time.Time

	defaultLogger = &Logger{level: LevelInfo, out: os.Stderr}
)

func init() {
	startOnce.Do(func() { startTime = time.Now() })
}

// Level represents a log severity.
type Level int

const (
	LevelDebug Level = iota - 1 // -1 so 0-level comparison works sanely
	LevelInfo
	LevelWarn
	LevelError
)

var levelLabels = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

// Logger is a simple, synchronous, human-readable logger.
type Logger struct {
	mu    sync.Mutex
	level Level
	out   io.Writer
}

// SetLogLevel sets the global minimum log level.
func SetLogLevel(raw string) {
	defaultLogger.SetLevel(ParseLogLevel(raw))
}

// ParseLogLevel converts a string to a Level.
func ParseLogLevel(raw string) Level {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return LevelDebug
	case "INFO", "":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

// SetLevel sets the logger's minimum level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Enabled reports whether the given level will be emitted.
func (l *Logger) Enabled(level Level) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return level >= l.level
}

// ---------------------------------------------------------------------------
// resource header helpers
// ---------------------------------------------------------------------------

// resHeader returns a compact resource snapshot: elapsed seconds, goroutines,
// approximate RSS (heap alloc MB).
func resHeader() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	secs := time.Since(startTime).Seconds()
	return fmt.Sprintf("[%.1fs go=%d mem=%dMB]",
		secs, runtime.NumGoroutine(), m.Alloc/(1024*1024))
}

// ---------------------------------------------------------------------------
// core emit
// ---------------------------------------------------------------------------

// log emits one line. component is optional (may be "").
func log(level Level, component, msg string) {
	if !defaultLogger.Enabled(level) {
		return
	}
	label := levelLabels[level]
	if label == "" {
		label = "INFO"
	}
	rh := resHeader()
	var line string
	if component != "" {
		line = fmt.Sprintf("%s %s - %s - %s\n", label, rh, component, msg)
	} else {
		line = fmt.Sprintf("%s %s - %s\n", label, rh, msg)
	}
	defaultLogger.mu.Lock()
	fmt.Fprint(defaultLogger.out, line)
	defaultLogger.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Public logging functions
//
// Each accepts an optional component (e.g. site name) and a printf-style
// message.  The component is extracted from a trailing string field named
// "site" if present, otherwise the first string field is used.  Fields are
// appended as space-separated key=value pairs.
// ---------------------------------------------------------------------------

// Debug emits a DEBUG-level line.
func Debug(msg string, fields map[string]any) {
	if fields == nil {
		log(LevelDebug, "", msg)
		return
	}
	component, rest := extractFields(fields)
	log(LevelDebug, component, appendFields(msg, rest))
}

// Info emits an INFO-level line.
func Info(msg string, fields map[string]any) {
	if fields == nil {
		log(LevelInfo, "", msg)
		return
	}
	component, rest := extractFields(fields)
	log(LevelInfo, component, appendFields(msg, rest))
}

// Warn emits a WARN-level line.
func Warn(msg string, fields map[string]any) {
	if fields == nil {
		log(LevelWarn, "", msg)
		return
	}
	component, rest := extractFields(fields)
	log(LevelWarn, component, appendFields(msg, rest))
}

// Error emits an ERROR-level line.
func Error(msg string, fields map[string]any) {
	if fields == nil {
		log(LevelError, "", msg)
		return
	}
	component, rest := extractFields(fields)
	log(LevelError, component, appendFields(msg, rest))
}

// SystemError emits an ERROR-level line marked as system.
func SystemError(msg string, fields map[string]any) {
	if fields == nil {
		log(LevelError, "system", msg)
		return
	}
	_, rest := extractFields(fields)
	log(LevelError, "system", appendFields(msg, rest))
}

// APIMiss emits a DEBUG-level line about missing API data.
// Downgraded from a separate level to DEBUG for simplicity.
func APIMiss(msg string, fields map[string]any) {
	if fields == nil {
		log(LevelDebug, "", msg)
		return
	}
	component, rest := extractFields(fields)
	log(LevelDebug, component, appendFields(msg, rest))
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// extractFields pulls a "site" or "component" key out of fields and returns
// it as the component, along with the remaining key-value pairs.
func extractFields(fields map[string]any) (component string, rest []string) {
	rest = make([]string, 0, len(fields))

	for k, v := range fields {
		// skip internal/reserved fields
		if k == "ts" || k == "level" {
			continue
		}
		// treat "site" / "component" as the log section header
		if component == "" && (k == "site" || k == "component") {
			if s := fmt.Sprintf("%v", v); s != "" {
				component = s
				continue
			}
		}
		rest = append(rest, fmt.Sprintf("%s=%v", k, v))
	}
	return
}

// appendFields concatenates msg with a formatted field list.
func appendFields(msg string, fields []string) string {
	if len(fields) == 0 {
		return msg
	}
	return msg + " " + strings.Join(fields, " ")
}
