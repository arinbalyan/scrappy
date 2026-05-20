package util

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelSystemError
	LevelAPIMiss
)

var levelNames = map[LogLevel]string{
	LevelDebug:       "DEBUG",
	LevelInfo:        "INFO",
	LevelWarn:        "WARN",
	LevelError:       "ERROR",
	LevelSystemError: "SYSTEM_ERROR",
	LevelAPIMiss:     "API_MISS",
}

type Logger struct {
	mu    sync.RWMutex
	level LogLevel
}

var defaultLogger = &Logger{level: LevelInfo}

func SetLogLevel(raw string) {
	defaultLogger.SetLevel(ParseLogLevel(raw))
}

func ParseLogLevel(raw string) LogLevel {
	s := strings.ToUpper(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	switch s {
	case "DEBUG":
		return LevelDebug
	case "INFO", "":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	case "SYSTEM_ERROR":
		return LevelSystemError
	case "API_MISS":
		return LevelAPIMiss
	default:
		return LevelInfo
	}
}

func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) Enabled(level LogLevel) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return level >= l.level
}

func Log(level LogLevel, msg string, fields map[string]any) {
	if !defaultLogger.Enabled(level) {
		return
	}
	rec := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": levelNames[level],
		"msg":   msg,
	}
	for k, v := range fields {
		rec[k] = v
	}
	b, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "{\"level\":\"SYSTEM_ERROR\",\"msg\":\"log marshal failed\",\"err\":%q}\n", err.Error())
		return
	}
	fmt.Fprintln(os.Stderr, string(b))
}

func Debug(msg string, fields map[string]any) { Log(LevelDebug, msg, fields) }
func Info(msg string, fields map[string]any)  { Log(LevelInfo, msg, fields) }
func Warn(msg string, fields map[string]any)  { Log(LevelWarn, msg, fields) }
func Error(msg string, fields map[string]any) { Log(LevelError, msg, fields) }
func SystemError(msg string, fields map[string]any) {
	Log(LevelSystemError, msg, fields)
}
func APIMiss(msg string, fields map[string]any) { Log(LevelAPIMiss, msg, fields) }
