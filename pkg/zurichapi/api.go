package zurichapi

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"time"
)

const (
	// Base URLs for the Zurich city council APIs
	GeschaeftBaseURL       = "https://www.gemeinderat-zuerich.ch/api/geschaeft"
	KontaktBaseURL         = "https://www.gemeinderat-zuerich.ch/api/kontakt"
	AbstimmungBaseURL      = "https://www.gemeinderat-zuerich.ch/api/abstimmung"
	BehoerdenmandatBaseURL = "https://www.gemeinderat-zuerich.ch/api/behoerdenmandat"
)

// FetchLatestGeschaeft fetches the most recent geschaeft from the Zurich council API
func (c *Client) FetchLatestGeschaeft() (*Geschaeft, error) {
	// Fetch recent geschaeft (council business) from 2024 onwards, sorted by date descending
	url := GeschaeftBaseURL + "/searchdetails?q=beginn_start%20%3E%20%222024-01-01%2000:00:00%22%20sortBy%20beginn_start/sort.descending&l=de-CH&s=1&m=1"

	body, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var resp GeschaeftSearchResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	if len(resp.Hits) == 0 {
		return nil, fmt.Errorf("no geschaeft found in API response")
	}

	return &resp.Hits[0].Geschaeft, nil
}

// FetchGeschaefteSince fetches up to limit Geschäfte whose Beginn — the
// sitting at which they were filed — falls after since, newest first.
//
// The query cannot filter by Geschaeftsart: PARIS exposes no search field for
// it, so callers filter the result.
func (c *Client) FetchGeschaefteSince(since time.Time, limit int) ([]Geschaeft, error) {
	q := fmt.Sprintf(`beginn_start > "%s" sortBy beginn_start/sort.descending`, since.Format("2006-01-02 15:04:05"))
	u := fmt.Sprintf("%s/searchdetails?q=%s&l=de-CH&s=1&m=%d", GeschaeftBaseURL, url.PathEscape(q), limit)

	body, err := c.makeRequest(u)
	if err != nil {
		return nil, err
	}

	var resp GeschaeftSearchResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	geschaefte := make([]Geschaeft, len(resp.Hits))
	for i, hit := range resp.Hits {
		geschaefte[i] = hit.Geschaeft
	}

	return geschaefte, nil
}

// FetchAllKontakte fetches all contacts from the Zurich council API
func (c *Client) FetchAllKontakte() ([]Kontakt, error) {
	url := KontaktBaseURL + "/searchdetails?q=seq>0&l=de-CH"

	body, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var resp KontaktSearchResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	contacts := make([]Kontakt, len(resp.Hits))
	for i, hit := range resp.Hits {
		contacts[i] = hit.Kontakt
	}

	return contacts, nil
}

// FetchRecentAbstimmungen fetches the n most recent abstimmungen (votes) from the Zurich council API
func (c *Client) FetchRecentAbstimmungen(limit int) ([]Abstimmung, error) {
	url := fmt.Sprintf("%s/searchdetails?q=seq%%3E0%%20sortBy%%20seq/sort.descending&l=de-CH&s=1&m=%d",
		AbstimmungBaseURL, limit)

	body, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var resp AbstimmungSearchResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	abstimmungen := make([]Abstimmung, len(resp.Hits))
	for i, hit := range resp.Hits {
		abstimmungen[i] = hit.Abstimmung
	}

	return abstimmungen, nil
}

// FetchAbstimmungenForSitzung fetches all votes for a specific session (Sitzung) by its GUID.
// The abstimmung API supports sitzungguid as a search field.
func (c *Client) FetchAbstimmungenForSitzung(sitzungGuid string) ([]Abstimmung, error) {
	url := fmt.Sprintf("%s/searchdetails?q=sitzungguid%%20any%%20%%22%s%%22&l=de-CH",
		AbstimmungBaseURL,
		sitzungGuid)

	body, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var resp AbstimmungSearchResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	var abstimmungen []Abstimmung
	for _, hit := range resp.Hits {
		abstimmungen = append(abstimmungen, hit.Abstimmung)
	}

	return abstimmungen, nil
}

// GetActiveMandates fetches active mandates from the Gemeinderat
// A mandate is considered active if its End date is "9999-12-31 00:00:00" or later
// excludeFunktionen is an optional list of Funktion values to exclude from results
func (c *Client) GetActiveMandates(excludeFunktionen ...string) ([]Behoerdenmandat, error) {
	// Query for active Gemeinderat mandates using API search fields
	// dauer_end >= "9999-12-31 00:00:00" filters for active mandates
	url := BehoerdenmandatBaseURL + "/searchdetails?q=gremium%20adj%20Gemeinderat%20and%20dauer_end%20%3E%3D%20%229999-12-31%2000:00:00%22&l=de-CH"

	body, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	// Strip the namespace declaration which causes issues with Go's XML parser
	body = bytes.ReplaceAll(body, []byte(` xmlns="http://www.cmiag.ch/cdws/Behoerdenmandat"`), []byte(""))

	var resp BehoerdenmandatSearchResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Build a set of excluded functions for efficient lookup
	excludeSet := make(map[string]bool)
	for _, f := range excludeFunktionen {
		excludeSet[f] = true
	}

	// Filter out excluded functions
	var activeMandates []Behoerdenmandat
	for _, hit := range resp.Hits {
		mandat := hit.Behoerdenmandat

		// Skip if this function is in the exclude list
		if excludeSet[mandat.Funktion] {
			continue
		}

		activeMandates = append(activeMandates, mandat)
	}

	return activeMandates, nil
}

// FetchActiveGemeinderatMandates fetches all active Gemeinderat member mandates
// This is a convenience function that excludes Stadtrat, Stimmenzählende, and Ratssekretariat roles
func (c *Client) FetchActiveGemeinderatMandates() ([]Behoerdenmandat, error) {
	return c.GetActiveMandates(
		"Stadtrat",
		"Stimmenzählende",
		"Ratssekretariat",
		"Präsidium Stadtrat",
		"Mitglied Stadtrat",
	)
}
