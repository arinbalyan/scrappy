package ats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

// ResolveSeeds returns company seed strings from env, config file, or search term.
func ResolveSeeds(searchTerm string, envKey string) (seeds []string, src SeedSource) {
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
			return seeds, SeedFromEnv
		}
	}

	// 2. Check config/company_slugs.yaml
	slugOnce.Do(loadSlugs)
	key := normalizeKey(envKey)
	for _, k := range aliasKeys(key) {
		if cfg, ok := slugDB[k]; ok && len(cfg) > 0 {
			return cfg, SeedFromConfig
		}
	}

	// 3. Fall back to search term
	if st := strings.TrimSpace(searchTerm); st != "" {
		return []string{st}, SeedFromSearch
	}
	return nil, SeedFromEnv
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
