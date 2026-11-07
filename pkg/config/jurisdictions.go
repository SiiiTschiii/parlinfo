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

	// NewSource builds the adapter that fetches this body's votes. Nil for a
	// body whose contacts are curated but whose votes no source serves; read it
	// through Source, which says so instead of panicking.
	NewSource func() votes.Source

	// NewMemberSource builds the adapter that enumerates this body's sitting
	// members. Kept separate from NewSource because the two are read on
	// different clocks: the bot fetches votes every few hours, while the roster
	// is fetched by a human running cmd/update_contacts after an election or a
	// Nachrücken. Nil for a body whose source publishes no roster.
	NewMemberSource func() votes.MemberSource

	// RosterOf names the jurisdiction whose members these are, for a chamber
	// registered once per post language. Its contacts are that one's, and it
	// is left out wherever the roster itself is the question — see
	// ContactsKey and Chambers. Empty for a chamber of its own.
	RosterOf string

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
// All three languages are listed because the label is not translated: every
// language's feed serves the same text, in whichever language it was entered.
// That has been French since the Wintersession 2025 and was Italian in
// 2018/19, which looks like the language of the presiding member — Marina
// Carobbio Guscetti's year — though nothing in the service says so. Matching
// any one language finds nothing for the years another was used, and the
// Italian labels, 66 Votazione finale and 128 Votazione sul complesso, are
// last seen in September 2019.
//
// A vote with no Subject at all — a third of them — is never one of these: in
// the Herbstsession 2026 every one was a motion, a postulate or an
// Ordnungsantrag. So a null Subject is left out rather than guessed at.
var nationalratSubjects = []string{
	"Schlussabstimmung",
	"Vote final",
	"Votazione finale",
	"Gesamtabstimmung",
	"Vote sur l'ensemble",
	"Votazione sul complesso",
}

// NationalratFrenchKey and NationalratItalianKey identify the Nationalrat
// posted in French and in Italian.
//
// They are jurisdictions of their own rather than a language setting on the
// Nationalrat, because a jurisdiction is what owns a vote log: a vote posted
// to the German accounts has not been posted to the French ones. The members
// are the same, so their contacts are the Nationalrat's — see RosterOf.
const (
	NationalratFrenchKey  = "nationalrat-fr"
	NationalratItalianKey = "nationalrat-it"
)

var nationalratFrench = votes.Jurisdiction{
	Key:       NationalratFrenchKey,
	Name:      "Conseil national",
	ShortName: "Conseil national",
	Language:  votes.French,
}

var nationalratItalian = votes.Jurisdiction{
	Key:       NationalratItalianKey,
	Name:      "Consiglio nazionale",
	ShortName: "Consiglio nazionale",
	Language:  votes.Italian,
}

// nationalratMaxAgeDays bounds the first run of every Nationalrat
// registration, since each one's log starts empty. Two weeks covers a
// session's final votes, which all fall on its last day, while keeping the
// previous session out of a run started early in the next.
const nationalratMaxAgeDays = 14

// nationalratIn registers the Nationalrat in a language other than German.
// The editorial policy and the age guard are the German registration's, so
// they cannot drift apart between accounts that post the same votes.
//
// No member source: the roster is the German registration's, kept with its
// tools. A second registration answering for the same people would make every
// member of the Nationalrat look, to prune, like someone who moved chambers.
func nationalratIn(j votes.Jurisdiction) Jurisdiction {
	return Jurisdiction{
		Jurisdiction: j,
		RosterOf:     NationalratKey,
		MaxAgeDays:   nationalratMaxAgeDays,
		NewSource: func() votes.Source {
			return parlamentch.New(j).WithSubjects(nationalratSubjects...)
		},
	}
}

// StaenderatKey identifies the Ständerat.
const StaenderatKey = "staenderat"

var staenderat = votes.Jurisdiction{
	Key:       StaenderatKey,
	Name:      "Ständerat",
	ShortName: "Ständerat",
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
		MaxAgeDays:   nationalratMaxAgeDays,
		NewSource: func() votes.Source {
			return parlamentch.New(nationalrat).WithSubjects(nationalratSubjects...)
		},
		NewMemberSource: func() votes.MemberSource { return parlamentch.New(nationalrat) },
	},
	NationalratFrenchKey:  nationalratIn(nationalratFrench),
	NationalratItalianKey: nationalratIn(nationalratItalian),
	// A roster and no votes. parlament.ch serves the Nationalrat's votes only,
	// so nothing can post the Ständerat yet; it is registered so its contacts
	// are kept with the same tools as every other body's, and so prune can tell
	// a Nationalrat who moved to the Ständerat from one who left.
	StaenderatKey: {
		Jurisdiction: staenderat,
		NewMemberSource: func() votes.MemberSource {
			return parlamentch.New(staenderat).WithCouncil(parlamentch.CouncilStaenderat)
		},
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
	return j, nil
}

// Source builds the adapter that fetches the body's votes, or says there is
// none.
func (j Jurisdiction) Source() (votes.Source, error) {
	if j.NewSource == nil {
		return nil, fmt.Errorf("%s has no vote source: its contacts are kept, its votes are not posted", j.Key)
	}
	return j.NewSource(), nil
}

// ContactsKey is the jurisdiction whose contacts file tags this one's posts:
// its own, or for a chamber registered once per language, the chamber's.
func (j Jurisdiction) ContactsKey() string {
	if j.RosterOf != "" {
		return j.RosterOf
	}
	return j.Key
}

// ContactsKeys is ContactsKey for several jurisdictions, each once, in order.
func ContactsKeys(js []Jurisdiction) []string {
	seen := map[string]bool{}
	var keys []string
	for _, j := range js {
		if k := j.ContactsKey(); !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys
}

// ChamberKeys lists the jurisdictions that are chambers of their own, leaving
// out the further languages a chamber is posted in. It is what a question
// about who sits where walks: asked of every registration, one person would
// sit in the Nationalrat three times.
func ChamberKeys() []string {
	var keys []string
	for _, k := range JurisdictionKeys() {
		if jurisdictions[k].RosterOf == "" {
			keys = append(keys, k)
		}
	}
	return keys
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
