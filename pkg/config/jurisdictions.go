// Package config is the composition root: it names the parliamentary bodies the
// bot covers, wires each to the API that serves it, and groups them into the
// accounts they post to.
//
// It is the only package that knows about every source, which keeps the sources
// unaware of each other and everything downstream unaware of all of them.
package config

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/openparldata"
	"github.com/siiitschiii/parlinfo/pkg/parlamentch"
	"github.com/siiitschiii/parlinfo/pkg/recapp"
	"github.com/siiitschiii/parlinfo/pkg/votes"
	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
)

// Jurisdiction is a body the bot posts for, plus the operational settings that
// are not properties of the body itself.
type Jurisdiction struct {
	votes.Jurisdiction

	// MaxAgeDays bounds how old a vote may be and still be posted.
	//
	// This is not a freshness preference. Dedup works by fetching the most
	// recent N votes and subtracting whatever the vote log records, and the
	// logs do not reach back to the beginning of time — so any vote older than
	// its log's earliest entry is indistinguishable from an unposted one. When
	// the source re-indexes an old vote back into the fetch window, this guard
	// is the only thing standing between that and a second posting.
	//
	// For a jurisdiction whose log starts empty, *every* historical vote looks
	// unposted, and this value alone bounds the first run.
	MaxAgeDays int

	// PostFrom, when set, is the first sitting date posted for this body, on
	// every platform. It is JURISDICTION_<KEY>_POST_FROM, as YYYY-MM-DD.
	//
	// Where MaxAgeDays is a window that slides forward, this is a fixed edge:
	// the body's vote logs start empty on it and nothing before it is ever
	// posted. That is what lets a body go live on an empty log without the log
	// claiming posts that were never made — the log records what was posted
	// and nothing else may write to it.
	PostFrom time.Time

	// NewSource builds the adapter that fetches this body's votes.
	NewSource func() votes.Source

	// NewMemberSource builds the adapter that enumerates this body's sitting
	// members. Kept separate from NewSource because the two are read on
	// different clocks: the bot fetches votes every few hours, while the roster
	// is fetched by a human running cmd/update_contacts after an election or a
	// Nachrücken. Nil for a body whose source publishes no roster.
	NewMemberSource func() votes.MemberSource

	// Enabled decides whether the scheduled run posts this body. It defaults to
	// false for every jurisdiction and is turned on with
	// JURISDICTION_<KEY>_ENABLED, e.g. JURISDICTION_ZURICH_CITY_ENABLED=true.
	//
	// Off by default because the consequence of the two mistakes is not
	// symmetric: a body that should post and does not is a visible gap someone
	// notices, while a body that should not post and does has already published
	// to a public account. Losing the variable stops the run with a non-zero
	// exit rather than quietly changing what gets published.
	//
	// Dry-run tools ignore this: previewing a body is exactly what you do
	// before enabling it.
	Enabled bool
}

// ZurichCantonKey identifies the Kantonsrat Zürich.
const ZurichCantonKey = "zurich-canton"

// zurichCanton is served by OpenParlData; PARIS covers only the city.
var zurichCanton = votes.Jurisdiction{
	Key:       ZurichCantonKey,
	Name:      "Kantonsrat Zürich",
	ShortName: "Kantonsrat ZH",
}

// zurichCantonBodyKey is the canton's body_key in OpenParlData.
const zurichCantonBodyKey = "ZH"

// cantonVoteDetails adapts the Kantonsrat's audio archive to the detail source
// OpenParlData asks for.
//
// The two packages are kept unaware of each other on purpose. OpenParlData
// serves twenty-odd cantons and must not grow a dependency on one body's
// archive, and the archive has no reason to know which API happens to need it.
// Joining them is this package's job.
type cantonVoteDetails struct{ *recapp.Client }

func (d cantonVoteDetails) Lookup(voteURLs map[string]string) (map[string]openparldata.VoteDetail, error) {
	found, err := d.Client.Lookup(voteURLs)
	out := make(map[string]openparldata.VoteDetail, len(found))
	for id, info := range found {
		out[id] = openparldata.VoteDetail{
			Type: info.Type,
		}
	}
	return out, err
}

// cantonLinks supplies the two pages the Kantonsrat publishes beside a Geschäft.
//
// It lives here rather than in either package it draws on, for the same reason
// cantonVoteDetails does: openparldata serves twenty-odd bodies and must not
// learn one parliament's URLs, and recapp has no reason to know which API needs
// them.
type cantonLinks struct{}

// ItemURL hands the widening straight to the archive, which owns the URL shape.
func (cantonLinks) ItemURL(voteURL string) string { return recapp.ItemURL(voteURL) }

// SessionURL points at the sitting list narrowed to one day.
//
// The Kantonsrat gives a sitting no permalink of its own: the list is a single
// page whose date filter lives in the query string, so a one-day range is the
// closest thing to an address a sitting has. It is a real link and not a guess
// — the site builds exactly this URL when a reader uses the filter — and it
// lands on the day's sittings with their Traktandenliste, Bulletin and
// recording. What it does not do is open them; a reader clicks the sitting.
//
// A day holding two sittings shows both, which is honest: the morning and
// afternoon of one day are one entry in every other respect the post makes.
func (cantonLinks) SessionURL(date time.Time) string {
	day := date.Format("2006-01-02")
	q := url.Values{}
	q.Set("startDate", day)
	q.Set("endDate", day)
	return "https://www.kantonsrat.zh.ch/ratsbetrieb/sitzungenundprotokolle/?" + q.Encode()
}

// NationalratKey identifies the Nationalrat.
const NationalratKey = "nationalrat"

var nationalrat = votes.Jurisdiction{
	Key:       NationalratKey,
	Name:      "Nationalrat",
	ShortName: "Nationalrat",
}

// nationalratSubjects is the editorial policy for the Nationalrat: final and
// overall votes only.
//
// The chamber takes around 300 votes a three-week session, up to 102 in a day,
// and most are on a single article or a procedural motion that means nothing
// to a reader on its own. A Gesamtabstimmung (the chamber's verdict on a whole
// bill) and a Schlussabstimmung (the last vote before a bill becomes law) are
// the ones that do: 25 and 18 of the 328 votes in the Herbstsession 2026.
//
// Both languages are listed because the German feed has served these labels in
// French since the Wintersession 2025. Matching the German alone finds nothing
// recent; matching the French alone would find nothing once that is fixed.
//
// A vote with no Subject at all — a third of them — is never one of these: in
// the Herbstsession 2026 every one was a motion, a postulate or an
// Ordnungsantrag. So a null Subject is left out rather than guessed at.
var nationalratSubjects = []string{
	"Schlussabstimmung",
	"Vote final",
	"Gesamtabstimmung",
	"Vote sur l'ensemble",
}

// jurisdictions is the registry, keyed by Jurisdiction.Key.
var jurisdictions = map[string]Jurisdiction{
	zurichapi.JurisdictionKey: {
		Jurisdiction:    zurichapi.Jurisdiction,
		MaxAgeDays:      90,
		NewSource:       func() votes.Source { return zurichapi.NewClient() },
		NewMemberSource: func() votes.MemberSource { return zurichapi.NewClient() },
	},
	ZurichCantonKey: {
		Jurisdiction: zurichCanton,
		// Far tighter than the city's 90 days, for two reasons. OpenParlData
		// harvests with a lag of up to ~4.2 days, so anything below that would
		// drop votes before they arrive. And the canton's log starts empty
		// against 2,626 historical votings, every one of which reads as
		// unposted — until the log is seeded, this number alone bounds what a
		// first run would publish. See cmd/seed_votelog.
		MaxAgeDays: 14,
		NewSource: func() votes.Source {
			return openparldata.New(zurichCanton, zurichCantonBodyKey).
				WithDetails(cantonVoteDetails{recapp.New()}).
				WithLinks(cantonLinks{})
		},
		NewMemberSource: func() votes.MemberSource {
			return openparldata.New(zurichCanton, zurichCantonBodyKey)
		},
	},
	NationalratKey: {
		Jurisdiction: nationalrat,
		// The log starts empty, so this alone bounds the first run. Two weeks
		// covers a session's final votes, which all fall on its last day, while
		// keeping the previous session out of a run started early in the next.
		MaxAgeDays: 14,
		NewSource: func() votes.Source {
			return parlamentch.New(nationalrat).WithSubjects(nationalratSubjects...)
		},
		NewMemberSource: func() votes.MemberSource { return parlamentch.New(nationalrat) },
	},
}

// LookupJurisdiction returns a registered jurisdiction, with MaxAgeDays
// overridden from the environment if configured.
func LookupJurisdiction(key string) (Jurisdiction, error) {
	j, ok := jurisdictions[key]
	if !ok {
		return Jurisdiction{}, fmt.Errorf("unknown jurisdiction %q", key)
	}
	j.MaxAgeDays = maxAgeDays(key, j.MaxAgeDays)
	j.Enabled = enabled(key, j.Enabled)
	postFrom, err := postFromDate(key)
	if err != nil {
		return Jurisdiction{}, err
	}
	j.PostFrom = postFrom
	return j, nil
}

// postFromDate resolves a jurisdiction's start date. Unset is no start date; a
// value that does not parse is an error rather than "no start date", because a
// mistyped bound would silently re-open the backlog it exists to keep closed.
func postFromDate(key string) (time.Time, error) {
	name := "JURISDICTION_" + envKey(key) + "_POST_FROM"
	raw := os.Getenv(name)
	if raw == "" {
		return time.Time{}, nil
	}
	d, err := ParseDate(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: %w", name, err)
	}
	return d, nil
}

// enabled resolves the posting switch for a jurisdiction.
func enabled(key string, fallback bool) bool {
	raw := os.Getenv("JURISDICTION_" + envKey(key) + "_ENABLED")
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

// JurisdictionKeys lists every registered jurisdiction, sorted for stable output.
func JurisdictionKeys() []string {
	keys := make([]string, 0, len(jurisdictions))
	for k := range jurisdictions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// maxAgeDays resolves the age guard for a jurisdiction, from
// MAX_VOTE_AGE_DAYS_<JURISDICTION>.
func maxAgeDays(key string, fallback int) int {
	if v, ok := envInt("MAX_VOTE_AGE_DAYS_" + envKey(key)); ok {
		return v
	}
	return fallback
}

// envKey turns a jurisdiction or channel key into the shape used in environment
// variable names: "zurich-city" becomes "ZURICH_CITY".
func envKey(key string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(key))
}

func envInt(name string) (int, bool) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}
