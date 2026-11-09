// Package openparldata adapts the OpenParlData API (https://api.openparldata.ch)
// to votes.Source.
//
// OpenParlData harmonises Swiss parliamentary data across cantons and the
// federal chambers, which makes one adapter enough for every body it covers.
// Data is CC BY 4.0; posts that credit the source use the Attribution string.
package openparldata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
	// Embeds the IANA zone database, so resolving Europe/Zurich does not depend
	// on the host having tzdata installed. Vote times are published to readers.
	_ "time/tzdata"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// DefaultBaseURL is the v1 API root.
const DefaultBaseURL = "https://api.openparldata.ch/v1"

// Attribution is the credit line required by OpenParlData's CC BY 4.0 licence.
const Attribution = "Daten: OpenParlData.ch"

// Client fetches votes for one body.
type Client struct {
	baseURL      string
	bodyKey      string
	jurisdiction votes.Jurisdiction
	http         *http.Client

	maxAttempts int
	retryDelay  time.Duration

	// votingIDs maps external_id to the API's numeric id, remembered from
	// whichever listing produced a vote so the sub-resource calls need no
	// extra round trip to rediscover it.
	votingIDs map[string]int64

	// sourceURLs maps external_id to the voting's own url_external_de,
	// remembered because enrichment overwrites Vote.SourceURL with the affair's
	// page and a DetailSource needs the original to identify the vote.
	sourceURLs map[string]string

	// authors caches an affair's first signatories, which enrichment would
	// otherwise ask for once per vote in a group. See Client.authorsOf.
	authors map[int64][]votes.Author

	// details optionally supplies the vote type and outcome this API omits.
	// Nil for every body that has no such source, which is all but one.
	details DetailSource

	// links optionally derives the archive and sitting pages a body publishes
	// alongside its business pages. Nil for a body that publishes neither.
	links LinkSource
}

// VoteDetail is what an out-of-band source knows about a voting beyond what
// this API serves. Every field is optional.
type VoteDetail struct {
	// Type is the vote type in the vocabulary the formatters speak.
	Type string
}

// DetailSource resolves vote details for votings this API describes
// incompletely.
//
// It takes every vote at once, keyed by external id and carrying that vote's
// source URL, so an implementation can batch its own requests however its API
// is shaped rather than being driven one vote at a time.
//
// Returning an error alongside a partial map is expected and handled: details
// are enrichment, and a vote without them keeps whatever the API gave it.
type DetailSource interface {
	Lookup(voteURLs map[string]string) (map[string]VoteDetail, error)
}

// WithDetails attaches a DetailSource. It returns the client so a jurisdiction
// can be wired in one expression.
func (c *Client) WithDetails(d DetailSource) *Client {
	c.details = d
	return c
}

// LinkSource derives the pages a body publishes beside the business page, from
// what this API already serves about a vote.
//
// It is an injected dependency rather than a table in this package because the
// URLs are properties of one parliament's website, and this adapter serves
// twenty-odd of them. Both methods return "" when the body has no such page,
// and callers must publish nothing rather than guessing a URL.
type LinkSource interface {
	// ItemURL widens a vote's own archive URL to the page covering every vote
	// of that agenda item.
	ItemURL(voteURL string) string
	// SessionURL returns the page for the sitting held on a date.
	SessionURL(date time.Time) string
}

// WithLinks attaches a LinkSource. It returns the client so a jurisdiction can
// be wired in one expression.
func (c *Client) WithLinks(l LinkSource) *Client {
	c.links = l
	return c
}

// New builds a client for one body. bodyKey is OpenParlData's body_key, e.g.
// "ZH" for the Kantonsrat Zürich or "261" for the Gemeinderat der Stadt Zürich.
func New(jurisdiction votes.Jurisdiction, bodyKey string) *Client {
	return &Client{
		baseURL:      DefaultBaseURL,
		bodyKey:      bodyKey,
		jurisdiction: jurisdiction,
		http:         &http.Client{Timeout: 30 * time.Second},
		maxAttempts:  3,
		retryDelay:   time.Second,
		votingIDs:    make(map[string]int64),
		sourceURLs:   make(map[string]string),
		authors:      make(map[int64][]votes.Author),
	}
}

// rememberVoting keeps the identifiers a voting's later calls need, which the
// listing that produced it is the only place to get them cheaply.
func (c *Client) rememberVoting(v votingDTO) {
	if v.ExternalID == "" {
		return
	}
	c.votingIDs[v.ExternalID] = v.ID
	if u := deref(v.URLExternalDe); u != "" {
		c.sourceURLs[v.ExternalID] = u
	}
}

// SetBaseURL points the client at another host. Used by tests to serve
// recorded fixtures instead of making live calls.
func (c *Client) SetBaseURL(u string) { c.baseURL = u }

// Jurisdiction returns the body this client serves.
func (c *Client) Jurisdiction() votes.Jurisdiction { return c.jurisdiction }

// get performs a GET and decodes the JSON body into out.
//
// lang_format=flat is set on every request without exception: without it the
// API returns localised fields as nested objects and every *_de field this
// adapter reads comes back null — silently, with a 200.
func (c *Client) get(path string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("lang_format", "flat")

	endpoint := c.baseURL + path + "?" + params.Encode()

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if attempt > 1 {
			// Linear backoff. OpenParlData publishes no rate limit, so this
			// errs towards being a well-behaved client rather than a fast one.
			time.Sleep(c.retryDelay * time.Duration(attempt-1))
		}

		body, retryable, err := c.doOnce(endpoint)
		if err != nil {
			lastErr = err
			if !retryable {
				break
			}
			continue
		}

		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("openparldata: decoding %s: %w", endpoint, err)
		}
		return nil
	}

	return fmt.Errorf("openparldata: %s: %w", endpoint, lastErr)
}

// doOnce performs one request, reporting whether a failure is worth retrying.
func (c *Client) doOnce(endpoint string) (body []byte, retryable bool, err error) {
	resp, err := c.http.Get(endpoint)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		return body, false, nil
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("status %d", resp.StatusCode)
	default:
		// 4xx other than 429 means the request is wrong; retrying cannot fix it.
		return nil, false, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
