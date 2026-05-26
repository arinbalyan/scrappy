package ats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/arinbalyan/scrappy/internal/util"
)

// SeedSource indicates where we got the company seed.
type SeedSource int

const (
	SeedFromEnv    SeedSource = iota // SCRAPPY_{PROVIDER}_SEEDS
	SeedFromSearch                   // SearchTerm used as company slug
)

// ResolveSeeds returns company seed strings from env or search term.
func ResolveSeeds(searchTerm string, envKey string) (seeds []string, src SeedSource) {
	env := os.Getenv(envKey)
	if strings.TrimSpace(env) != "" {
		for _, s := range strings.Split(env, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				seeds = append(seeds, s)
			}
		}
	}
	if len(seeds) > 0 {
		return seeds, SeedFromEnv
	}
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
