package util

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ── Level ────────────────────────────────────────────────────────────────────

type Level int

const (
	LevelDebug Level = iota - 1
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

// ── Logger ────────────────────────────────────────────────────────────────────

type Logger struct {
	mu    sync.Mutex
	level Level
	out   io.Writer
}

var (
	startTime     time.Time
	defaultLogger = &Logger{level: LevelInfo, out: os.Stderr}
)

func init() {
	startTime = time.Now()
}

func SetLogLevel(raw string) {
	defaultLogger.SetLevel(ParseLogLevel(raw))
}

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

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) Enabled(level Level) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return level >= l.level
}

// ── Emit ──────────────────────────────────────────────────────────────────────

var isTerminal = func() bool {
	fi, err := os.Stderr.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}()

const reset = "\033[0m"

var levelColor = map[Level]string{
	LevelDebug: "\033[90m", // grey
	LevelInfo:  "\033[34m", // blue
	LevelWarn:  "\033[33m", // yellow
	LevelError: "\033[31m", // red
}

func labelColor(level Level) string {
	if !isTerminal {
		return ""
	}
	c, ok := levelColor[level]
	if !ok {
		return "\033[34m"
	}
	return c
}

func relSince(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%.0fm%02ds", d.Minutes(), int(d.Seconds())%60)
	}
}

func emit(level Level, component string, msg string, fields map[string]any) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	if level < defaultLogger.level {
		return
	}

	label := levelLabels[level]
	elapsed := relSince(startTime)

	// Build:  LEVEL  [elapsed go=N mem=N]  component  message  k=v k=v
	var header string
	if isTerminal {
		header = fmt.Sprintf("%s%-5s%s %s%-8s%s",
			labelColor(level), label, reset,
			"\033[90m", elapsed, reset,
		)
		if component != "" {
			header += fmt.Sprintf(" \033[36m%s\033[0m", component)
		}
	} else {
		header = fmt.Sprintf("%-5s %s", label, elapsed)
		if component != "" {
			header += fmt.Sprintf(" %s", component)
		}
	}
	line := header
	if msg != "" {
		line += fmt.Sprintf(" %s", msg)
	}
	if len(fields) > 0 {
		// Sort keys for stable output
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		for _, k := range keys {
			v := fields[k]
			if isTerminal {
				line += fmt.Sprintf(" \033[90m%s=%v\033[0m", k, v)
			} else {
				line += fmt.Sprintf(" %s=%v", k, v)
			}
		}
	}
	fmt.Fprintln(defaultLogger.out, line)
}

// ── Public API ────────────────────────────────────────────────────────────────

func Debug(msg string, fields map[string]any) {
	if !defaultLogger.Enabled(LevelDebug) {
		return
	}
	component, rest := extractFields(fields)
	emit(LevelDebug, component, msg, rest)
}

func Info(msg string, fields map[string]any) {
	if !defaultLogger.Enabled(LevelInfo) {
		return
	}
	component, rest := extractFields(fields)
	emit(LevelInfo, component, msg, rest)
}

func Warn(msg string, fields map[string]any) {
	if !defaultLogger.Enabled(LevelWarn) {
		return
	}
	component, rest := extractFields(fields)
	emit(LevelWarn, component, msg, rest)
}

func Error(msg string, fields map[string]any) {
	if !defaultLogger.Enabled(LevelError) {
		return
	}
	component, rest := extractFields(fields)
	emit(LevelError, component, msg, rest)
}

func SystemError(msg string, fields map[string]any) {
	if !defaultLogger.Enabled(LevelError) {
		return
	}
	_, rest := extractFields(fields)
	emit(LevelError, "system", msg, rest)
}

func APIMiss(msg string, fields map[string]any) {
	// Downgraded to DEBUG
	if !defaultLogger.Enabled(LevelDebug) {
		return
	}
	component, rest := extractFields(fields)
	emit(LevelDebug, component, "api_miss: "+msg, rest)
}

// ── Field helpers ─────────────────────────────────────────────────────────────

func extractFields(fields map[string]any) (component string, rest map[string]any) {
	if fields == nil {
		return "", nil
	}
	if s, ok := fields["site"]; ok {
		if str, ok := s.(string); ok {
			component = str
		}
	}
	if component == "" {
		for k, v := range fields {
			if str, ok := v.(string); ok {
				component = str
				delete(fields, k)
				break
			}
		}
	}
	if _, ok := fields["site"]; ok {
		delete(fields, "site")
	}
	return component, fields
}
