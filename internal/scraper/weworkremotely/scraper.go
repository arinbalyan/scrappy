package weworkremotely

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultFeedURL = "https://weworkremotely.com/remote-job-rss-feed"

type Scraper struct {
	client  *http.Client
	feedURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, feedURL: defaultFeedURL}
}

func NewWithFeedURL(client *http.Client, u string) *Scraper {
	s := New(client)
	if strings.TrimSpace(u) != "" {
		s.feedURL = strings.TrimSpace(u)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteWeWorkRemotely }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wwr request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wwr status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("wwr read: %w", err)
	}
	out, err := parseFeedItems(body, input.ResultsWanted)
	if err != nil {
		return nil, fmt.Errorf("wwr parse: %w", err)
	}
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("weworkremotely no parseable jobs")
	}
	return out, nil
}

func splitTitle(v string) (string, string) {
	v = strings.TrimSpace(v)
	parts := strings.Split(v, ":")
	if len(parts) < 2 {
		return v, ""
	}
	company := strings.TrimSpace(parts[0])
	title := strings.TrimSpace(strings.Join(parts[1:], ":"))
	return title, company
}

func sanitizeXMLAmpersands(in []byte) []byte {
	out := make([]byte, 0, len(in)+32)
	for i := 0; i < len(in); i++ {
		ch := in[i]
		if ch != '&' {
			out = append(out, ch)
			continue
		}
		remaining := in[i:]
		if bytes.HasPrefix(remaining, []byte("&amp;")) || bytes.HasPrefix(remaining, []byte("&lt;")) || bytes.HasPrefix(remaining, []byte("&gt;")) || bytes.HasPrefix(remaining, []byte("&quot;")) || bytes.HasPrefix(remaining, []byte("&apos;")) || bytes.HasPrefix(remaining, []byte("&#")) {
			out = append(out, ch)
			continue
		}
		out = append(out, []byte("&amp;")...)
	}
	return out
}

func parseFeedItems(body []byte, resultsWanted int) ([]model.JobPost, error) {
	items := parseFeedItemsXML(body)
	if len(items) == 0 {
		items = parseFeedItemsRegex(body)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no feed items")
	}
	limit := resultsWanted
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		it := items[i]
		title, company := splitTitle(it.Title)
		post := model.JobPost{
			ID:          fmt.Sprintf("wwr-%d-%s", i+1, util.NormalizeSlug(title)),
			Title:       title,
			CompanyName: company,
			JobURL:      strings.TrimSpace(it.Link),
			Description: strings.TrimSpace(it.Description),
			IsRemote:    true,
		}
		post.DatePosted = util.ParseDatePosted(it.PubDate)
		if post.JobURL == "" {
			post.JobURL = "https://weworkremotely.com/"
		}
		out = append(out, post)
	}
	return out, nil
}

type feedItem struct {
	Title       string
	Link        string
	Description string
	PubDate     string
}

func parseFeedItemsXML(body []byte) []feedItem {
	sanitized := sanitizeXMLAmpersands(body)
	var feed struct {
		Channel struct {
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				Description string `xml:"description"`
				PubDate     string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.NewDecoder(bytes.NewReader(sanitized)).Decode(&feed); err != nil {
		return nil
	}
	out := make([]feedItem, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		out = append(out, feedItem{Title: it.Title, Link: it.Link, Description: it.Description, PubDate: it.PubDate})
	}
	return out
}

var (
	reItemBlock = regexp.MustCompile(`(?is)<item\b[^>]*>(.*?)</item>`)
	reTag       = regexp.MustCompile(`(?is)<%s\b[^>]*>(.*?)</%s>`)
)

func parseFeedItemsRegex(body []byte) []feedItem {
	s := string(body)
	blocks := reItemBlock.FindAllStringSubmatch(s, -1)
	if len(blocks) == 0 {
		return nil
	}
	out := make([]feedItem, 0, len(blocks))
	for _, b := range blocks {
		blk := b[1]
		it := feedItem{
			Title:       extractTag(blk, "title"),
			Link:        extractTag(blk, "link"),
			Description: extractTag(blk, "description"),
			PubDate:     extractTag(blk, "pubDate"),
		}
		if strings.TrimSpace(it.Title) == "" && strings.TrimSpace(it.Link) == "" {
			continue
		}
		out = append(out, it)
	}
	return out
}

func extractTag(block, tag string) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?is)<%s\\b[^>]*>(.*?)</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
	m := re.FindStringSubmatch(block)
	if len(m) < 2 {
		return ""
	}
	v := strings.TrimSpace(m[1])
	v = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(v, "")
	return strings.TrimSpace(html.UnescapeString(v))
}
