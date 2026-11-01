// Package parlamentch adapts the Parlamentsdienste's OData service
// (https://ws.parlament.ch/odata.svc) to votes.Source, for the Nationalrat.
//
// It is a separate adapter rather than one more body in pkg/openparldata
// because OpenParlData's Nationalrat rows stop at 2025-09-26: the official
// service is the only federal source that is current. The Ständerat is out of
// scope, because it publishes name lists for only some of its votes by design.
package parlamentch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	// Embeds the IANA zone database, so resolving Europe/Zurich does not depend
	// on the host having tzdata installed. Vote times are published to readers.
	_ "time/tzdata"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// DefaultBaseURL is the OData service root.
const DefaultBaseURL = "https://ws.parlament.ch/odata.svc"

// Attribution credits the Parlamentsdienste. Their open data terms allow use
// only with the source named as «Parlamentsdienste der Bundesversammlung,
// Bern», in those words:
// https://www.parlament.ch/de/über-das-parlament/fakten-und-zahlen/open-data-web-services
const Attribution = "Daten: Parlamentsdienste der Bundesversammlung, Bern"

// Client fetches Nationalrat votes.
type Client struct {
	baseURL      string
	jurisdiction votes.Jurisdiction
	http         *http.Client

	maxAttempts int
	retryDelay  time.Duration

	// subjects restricts FetchRecent to votes whose Subject starts with one of
	// these, compared case-insensitively. Empty means every vote. See
	// WithSubjects.
	subjects []string

	// businessTypes caches each business's type, which every vote of a group
	// would otherwise ask for separately.
	businessTypes map[int64]string

	// multiBill caches whether a business holds more than one bill, by
	// BusinessNumber. See Client.hasSeveralBills.
	multiBill map[int64]bool

	// authors caches who filed each business, by BusinessNumber. See
	// Client.authorsOf.
	authors map[int64][]votes.Author

	// labels holds what the listing says about each vote that votes.Vote has
	// no field for, keyed by SourceID. See Client.label.
	labels map[string]voteLabel

	// council is the chamber whose roster FetchMembers reads. See WithCouncil.
	council int
}

// New builds a client for the Nationalrat.
func New(jurisdiction votes.Jurisdiction) *Client {
	return &Client{
		baseURL:       DefaultBaseURL,
		jurisdiction:  jurisdiction,
		http:          &http.Client{Timeout: 60 * time.Second},
		maxAttempts:   3,
		retryDelay:    2 * time.Second,
		businessTypes: make(map[int64]string),
		labels:        make(map[string]voteLabel),
		multiBill:     make(map[int64]bool),
		authors:       make(map[int64][]votes.Author),
		council:       CouncilNationalrat,
	}
}

// WithSubjects restricts the votes fetched to those whose Subject starts with
// one of the given labels, ignoring case. It returns the client so a
// jurisdiction can be wired in one expression.
//
// This is where the editorial policy is applied, and it is applied on the
// server: the Nationalrat takes around 300 votes a session, most of them on
// single articles or procedural motions that mean nothing in isolation, so the
// filter has to sit in front of the fetch window rather than behind it. Behind
// it, a busy sitting day would fill the window with article votes and push the
// day's final votes out of it.
//
// Prefix rather than equality, because the field is typed by hand: alongside
// 1,582 plain "Gesamtabstimmung" the service holds "Gesamtabstimmung zur
// Vorlage 2", "Gesamtabstimmung BB 2" and trailing spaces.
func (c *Client) WithSubjects(subjects ...string) *Client {
	c.subjects = subjects
	return c
}

// SetBaseURL points the client at another host. Used by tests to serve
// recorded fixtures instead of making live calls.
func (c *Client) SetBaseURL(u string) { c.baseURL = u }

// Jurisdiction returns the body this client serves.
func (c *Client) Jurisdiction() votes.Jurisdiction { return c.jurisdiction }

// query is an OData query string. Its values are encoded by hand because
// url.Values encodes a space as "+", which the service does not read back as a
// space inside a $filter.
type query map[string]string

func (q query) encode() string {
	parts := make([]string, 0, len(q))
	for _, k := range []string{"$filter", "$orderby", "$top", "$skip", "$select"} {
		if v, ok := q[k]; ok {
			parts = append(parts, k+"="+url.PathEscape(v))
		}
	}
	parts = append(parts, "$format=json")
	return strings.Join(parts, "&")
}

// get performs a GET and decodes the rows of an OData collection into out,
// which must be a pointer to a slice.
//
// The service answers in two shapes. A query with $top and $orderby returns
// {"d": [...]}, while a plain $filter returns {"d": {"results": [...]}}, and
// which one a given request gets is not documented. Both are accepted.
func (c *Client) get(path string, q query, out any) error {
	endpoint := c.baseURL + path + "?" + q.encode()

	body, err := c.fetch(endpoint)
	if err != nil {
		return err
	}

	var envelope struct {
		D json.RawMessage `json:"d"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("parlamentch: decoding %s: %w", endpoint, err)
	}
	rows := envelope.D
	if trimmed := strings.TrimSpace(string(rows)); strings.HasPrefix(trimmed, "{") {
		var wrapped struct {
			Results json.RawMessage `json:"results"`
		}
		if err := json.Unmarshal(rows, &wrapped); err != nil {
			return fmt.Errorf("parlamentch: decoding %s: %w", endpoint, err)
		}
		rows = wrapped.Results
	}
	if err := json.Unmarshal(rows, out); err != nil {
		return fmt.Errorf("parlamentch: decoding %s: %w", endpoint, err)
	}
	return nil
}

// getOne performs a GET for a single entity, which the service always wraps
// as {"d": {...}}.
func (c *Client) getOne(path string, q query, out any) error {
	endpoint := c.baseURL + path + "?" + q.encode()

	body, err := c.fetch(endpoint)
	if err != nil {
		return err
	}
	var envelope struct {
		D json.RawMessage `json:"d"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("parlamentch: decoding %s: %w", endpoint, err)
	}
	if err := json.Unmarshal(envelope.D, out); err != nil {
		return fmt.Errorf("parlamentch: decoding %s: %w", endpoint, err)
	}
	return nil
}

// count performs a GET on an entity set's $count, which answers a bare integer
// rather than the usual JSON envelope, and does not take $format.
func (c *Client) count(path, filter string) (int, error) {
	endpoint := c.baseURL + path + "/$count?$filter=" + url.PathEscape(filter)

	body, err := c.fetch(endpoint)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(string(body), "\ufeff")))
	if err != nil {
		return 0, fmt.Errorf("parlamentch: %s: not a count: %q", endpoint, truncate(string(body), 40))
	}
	return n, nil
}

// fetch retries what is worth retrying. The service is slow under load and
// answers an expensive query with a 500 rather than a 429, so both count.
func (c *Client) fetch(endpoint string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(c.retryDelay * time.Duration(attempt-1))
		}

		body, retryable, err := c.doOnce(endpoint)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retryable {
			break
		}
	}
	return nil, fmt.Errorf("parlamentch: %s: %w", endpoint, lastErr)
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
		return nil, false, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// zurich is the zone sitting times are published in.
var zurich = mustLoadLocation("Europe/Zurich")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// parseODataDate reads the service's "/Date(1790933358735)/" form. It returns
// the zero time for anything else, which callers treat as "unknown".
//
// The count is not what it claims to be. It reads as milliseconds since the
// epoch in UTC, but it is the Zürich wall clock written as though it were UTC:
// the Abstimmungsprotokoll of vote 33215 is stamped 09:29:18, and its
// VoteEnd, read as UTC, is 2026-10-02 09:29:18. Converting it to Zürich time
// would date every vote two hours late in summer and one in winter. So the
// fields are read off in UTC and the same wall clock is placed in Zürich.
//
// The "+0120" some fields append is ignored for the same reason: it is no
// offset that has ever applied in Switzerland.
func parseODataDate(s string) time.Time {
	inner, ok := strings.CutPrefix(s, "/Date(")
	if !ok {
		return time.Time{}
	}
	inner, ok = strings.CutSuffix(inner, ")/")
	if !ok || inner == "" {
		return time.Time{}
	}
	if i := strings.IndexAny(inner[1:], "+-"); i >= 0 {
		inner = inner[:i+1]
	}
	ms, err := strconv.ParseInt(inner, 10, 64)
	if err != nil {
		return time.Time{}
	}
	w := time.UnixMilli(ms).UTC()
	return time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), w.Nanosecond(), zurich)
}
