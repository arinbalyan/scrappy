package util

import (
	"hash/fnv"
	"strconv"
	"strings"
	"time"
)

// HashID generates a stable FNV-64a hash string for deduplication.
func HashID(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return strconv.FormatUint(h.Sum64(), 10)
}

func NormalizeSlug(v string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(v), " ", "-"))
}

func ParseDatePosted(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return &t
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	return nil
}
