package ats

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

//go:embed company_slugs.toml
var embeddedSlugs embed.FS

// SlugFile is the path to the company slugs TOML file.
const SlugFile = "config/company_slugs.toml"

var (
	slugDB   map[string][]string
	slugOnce sync.Once

	// slugStaleness tracks consecutive 0-job runs per company slug.
	// Used by scrapers to deprioritize dead slugs over time.
	slugStaleness   map[string]int
	slugStalenessMu sync.Mutex
)

const StalenessThreshold = 3 // consecutive 0-job runs before a slug is considered stale

// loadSlugs reads the company slugs file once, falling back to embedded data.
func loadSlugs() {
	raw, _ := embeddedSlugs.ReadFile("company_slugs.toml")

	// Try file override (users can supply their own config/company_slugs.toml)
	if fileRaw, err := os.ReadFile(SlugFile); err == nil {
		raw = fileRaw
	}

	if len(raw) == 0 {
		return
	}
	var data map[string][]string
	if err := toml.Unmarshal(raw, &data); err != nil {
		return
	}
	slugDB = data
}

// SeedSource indicates where we got the company seed.
type SeedSource int

const (
	SeedFromEnv    SeedSource = iota // SCRAPPY_{PROVIDER}_SEEDS
	SeedFromConfig                   // config/company_slugs.toml
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

// aliasKeys provides fallback key names used in company_slugs.toml.
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

	// 2. Check config/company_slugs.toml
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

// ProcessSeeds concurrently fetches job data for multiple ATS company slugs.
// It fans out seeds across nWorkers goroutines, calling fetchFn for each slug.
// fetchFn returns jobs and a bool indicating whether to continue to the next slug.
// Results are collected in order but fetched concurrently.
func ProcessSeeds(ctx context.Context, seeds []string, nWorkers int, wanted int, fetchFn func(ctx context.Context, slug string) ([]model.JobPost, error)) []model.JobPost {
	if nWorkers <= 0 {
		nWorkers = 3
	}
	if len(seeds) == 0 {
		return nil
	}

	type seedResult struct {
		jobs []model.JobPost
	}

	seedCh := make(chan string, len(seeds))
	for _, s := range seeds {
		seedCh <- s
	}
	close(seedCh)

	resultCh := make(chan seedResult, len(seeds))
	var wg sync.WaitGroup

	sem := make(chan struct{}, nWorkers)
	for range seeds {
		sem <- struct{}{}
	}
	close(sem)

	for s := range seedCh {
		select {
		case <-ctx.Done():
			wg.Wait()
			close(resultCh)
			// Collect whatever we have
			var all []model.JobPost
			for r := range resultCh {
				all = append(all, r.jobs...)
				if wanted > 0 && len(all) >= wanted {
					return all[:wanted]
				}
			}
			return all
		default:
		}

		wg.Add(1)
		go func(slug string) {
			defer wg.Done()
			jobs, err := fetchFn(ctx, slug)
			if err != nil {
				return
			}
			if len(jobs) > 0 {
				resultCh <- seedResult{jobs: jobs}
			}
		}(s)
	}

	wg.Wait()
	close(resultCh)

	var all []model.JobPost
	for r := range resultCh {
		all = append(all, r.jobs...)
		if wanted > 0 && len(all) >= wanted {
			return all[:wanted]
		}
	}
	return all
}

// MarkStale increments the staleness counter for a company slug.
// Call this when a slug returns 0 jobs — after StalenessThreshold consecutive
// failures the slug is reported by StaleSlugs().
func MarkStale(slug string) {
	slugStalenessMu.Lock()
	defer slugStalenessMu.Unlock()
	if slugStaleness == nil {
		slugStaleness = make(map[string]int)
	}
	slugStaleness[slug]++
}

// MarkFresh resets the staleness counter when a slug starts returning jobs again.
func MarkFresh(slug string) {
	slugStalenessMu.Lock()
	defer slugStalenessMu.Unlock()
	delete(slugStaleness, slug)
}

// StaleSlugs returns all slugs whose consecutive 0-job count
// has reached StalenessThreshold. Consumers can use this to update
// company_slugs.toml or exclude dead slugs from future runs.
func StaleSlugs() []string {
	slugStalenessMu.Lock()
	defer slugStalenessMu.Unlock()
	var out []string
	for slug, n := range slugStaleness {
		if n >= StalenessThreshold {
			out = append(out, slug)
		}
	}
	return out
}
