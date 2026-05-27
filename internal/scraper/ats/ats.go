package ats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/arinbalyan/scrappy/internal/util"
)

// SlugFile is the path to the company slugs YAML file.
const SlugFile = "config/company_slugs.yaml"

var (
	slugDB   map[string][]string
	slugOnce sync.Once
)

// loadSlugs reads the company slugs file once.
func loadSlugs() {
	slugDB = make(map[string][]string)
	raw, err := os.ReadFile(SlugFile)
	if err != nil {
		return // file not found — env/search only
	}
	var data map[string][]string
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return
	}
	slugDB = data
}

// SeedSource indicates where we got the company seed.
type SeedSource int

const (
	SeedFromEnv    SeedSource = iota // SCRAPPY_{PROVIDER}_SEEDS
	SeedFromConfig                   // config/company_slugs.yaml
	SeedFromSearch                   // SearchTerm used as company slug
)

const defaultMaxATSSeeds = 20

// SeedSourceString returns a stable string label for logs/metrics.
func SeedSourceString(src SeedSource) string {
	switch src {
	case SeedFromEnv:
		return "env"
	case SeedFromConfig:
		return "config"
	case SeedFromSearch:
		return "search"
	default:
		return "unknown"
	}
}

// normalizeKey converts "SCRAPPY_LEVER_SEEDS" to "lever" for slug lookup.
func normalizeKey(envKey string) string {
	k := strings.TrimPrefix(envKey, "SCRAPPY_")
	k = strings.TrimSuffix(k, "_SEEDS")
	return strings.ToLower(k)
}

// aliasKeys provides fallback key names used in company_slugs.yaml.
func aliasKeys(key string) []string {
	aliases := []string{key}
	switch key {
	case "breezyhr":
		aliases = append(aliases, "breezy")
	case "joincom":
		aliases = append(aliases, "join")
	case "oracle":
		aliases = append(aliases, "oracle-hcm")
	}
	return aliases
}

func maxATSSeeds() int {
	raw := strings.TrimSpace(os.Getenv("SCRAPPY_ATS_MAX_SEEDS"))
	if raw == "" {
		return defaultMaxATSSeeds
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultMaxATSSeeds
	}
	return n
}

// ResolveSeedsWithMeta returns company seed strings with source and pre-cap count.
func ResolveSeedsWithMeta(searchTerm string, envKey string) (seeds []string, src SeedSource, originalCount int) {
	// 1. Check env var (overrides everything)
	env := os.Getenv(envKey)
	if strings.TrimSpace(env) != "" {
		for _, s := range strings.Split(env, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				seeds = append(seeds, s)
			}
		}
		if len(seeds) > 0 {
			src = SeedFromEnv
			originalCount = len(seeds)
			maxSeeds := maxATSSeeds()
			if len(seeds) > maxSeeds {
				seeds = seeds[:maxSeeds]
			}
			return seeds, src, originalCount
		}
	}

	// 2. Check config/company_slugs.yaml
	slugOnce.Do(loadSlugs)
	key := normalizeKey(envKey)
	for _, k := range aliasKeys(key) {
		cfg, exists := slugDB[k]
		if exists {
			if len(cfg) > 0 {
				seeds = cfg
				src = SeedFromConfig
				originalCount = len(seeds)
				maxSeeds := maxATSSeeds()
				if len(seeds) > maxSeeds {
					seeds = seeds[:maxSeeds]
				}
				return seeds, src, originalCount
			}
			// Config key exists but empty (e.g. [] — marked dead). Do not fall back to search term.
			return nil, SeedFromConfig, 0
		}
	}

	// 3. Fall back to search term (only when no config entry exists at all)
	if st := strings.TrimSpace(searchTerm); st != "" {
		return []string{st}, SeedFromSearch, 1
	}
	return nil, SeedFromEnv, 0
}

// ResolveSeeds returns company seed strings from env, config file, or search term.
func ResolveSeeds(searchTerm string, envKey string) (seeds []string, src SeedSource) {
	seeds, src, _ = ResolveSeedsWithMeta(searchTerm, envKey)
	return seeds, src
}

// FetchJSON fetches a URL and decodes JSON into the target.
func FetchJSON(ctx context.Context, client *http.Client, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

// BuildID creates a prefixed ID with the given provider prefix and slug+jobID hash.
func BuildID(prefix string, slug string, jobID string) string {
	raw := slug + "-" + jobID
	return prefix + "-" + util.HashID(raw)
}
