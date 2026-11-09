package openparldata

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// dateLayouts are the timestamp shapes OpenParlData has been observed to return.
var dateLayouts = []string{"2006-01-02T15:04:05", time.RFC3339, "2006-01-02"}

// dateOnlyLayout is the one shape above that carries no time of day.
const dateOnlyLayout = "2006-01-02"

// bodyLocation is the wall clock every body this adapter serves actually sits by.
//
// OpenParlData publishes timestamps in UTC with no zone marker, while the
// official archives and any reader use local time: a vote the Kantonsrat's own
// archive lists at 11:29 arrives here as 09:29. Posts have to agree with the
// archive they link to, or the time in a post sends a reader to the wrong entry
// — and a vote taken late in a sitting would otherwise be grouped under the
// previous day.
var bodyLocation = loadBodyLocation()

func loadBodyLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		// Falls back to a fixed winter offset rather than silently reverting to
		// UTC, which would be an hour further out for half the year.
		return time.FixedZone("CET", 60*60)
	}
	return loc
}

// toVote maps a voting to the neutral model, without the member votes and
// affair number that need further calls (see Client.enrich).
func (c *Client) toVote(v votingDTO) votes.Vote {
	date := parseDate(v.Date)

	title := deref(v.AffairTitleDe)
	subtitle := deref(v.TitleDe)
	if title == "" {
		// Some bodies title the voting but not the affair.
		title, subtitle = subtitle, ""
	}
	if sameHeadline(subtitle, title) {
		// Kanton Zürich has no agenda-item concept and repeats the affair title
		// as the voting title. A subtitle that restates the headline adds
		// nothing and would render as "X: X".
		subtitle = ""
	}

	out := votes.Vote{
		SourceID:     v.ExternalID,
		Jurisdiction: c.jurisdiction.Key,
		Body:         c.jurisdiction.ShortName,
		Date:         date,
		// OpenParlData has no per-sitting sequence number, but its timestamps
		// are second-precision, so they order votes within a sitting exactly.
		Sequence: sequenceFromDate(date),

		Title:    title,
		Subtitle: subtitle,
		Type:     deref(v.TypeDe),

		SourceURL: deref(v.URLExternalDe),

		Yes:        v.ResultsYes,
		No:         v.ResultsNo,
		Abstention: v.ResultsAbstention,
		Absent:     v.ResultsAbsent,

		Decision: decision(v),

		// CC BY 4.0 obliges us to credit the source wherever the data appears.
		Attribution: Attribution,

		Affair: votes.Affair{
			Title:  deref(v.AffairTitleDe),
			Number: groupingNumber(v.AffairID, v.ExternalID),
		},
	}

	if v.AffairID != nil {
		out.Affair.ID = strconv.FormatInt(*v.AffairID, 10)
	}
	// Provisional until the affair is fetched, which replaces it.
	out.GroupURL = out.SourceURL

	if c.links != nil {
		// The archive link is kept in its own field because enrichment is about
		// to overwrite SourceURL with the business page, and a post carries
		// both.
		//
		// It is filled only for a body with a LinkSource even though url_external_de
		// needs no deriving, because publishing it is a claim about what it
		// opens: posts label this link "Video", which is true of the
		// Kantonsrat's audio archive and not of whatever page the other twenty-odd
		// bodies put there. A LinkSource is how a body says its archive is
		// understood.
		out.VideoURL = out.SourceURL
		out.VideoGroupURL = c.links.ItemURL(out.VideoURL)
		if !date.IsZero() {
			// A vote whose date would not parse has no sitting to point at.
			// Guessing one would send a reader to an empty list.
			out.SessionURL = c.links.SessionURL(date)
		}
	}

	return out
}

// applyAffair fills in the fields that only the /affairs call provides.
func applyAffair(v *votes.Vote, a affairDTO) {
	if n := deref(a.Number); n != "" {
		v.Affair.Number = n
	}
	if t := deref(a.TypeNameDe); t != "" {
		v.Affair.Type = t
	}
	if t := deref(a.TitleDe); t != "" {
		v.Affair.Title = t
		if v.Title == "" {
			v.Title = t
		}
	}
	if u := deref(a.URLExternalDe); u != "" {
		v.Affair.URL = u

		// Every post about this business leads with the parliament's own
		// Geschäft page, whether it covers one vote or five.
		//
		// It is not the page where a tally is easiest to read — that is the
		// vote's own archive page, and this one gives the totals in prose with
		// a name list for only some votes. It is chosen for what a published
		// link has to survive: it is the canton's own permalink, keyed by the
		// Geschäft id, on the parliament's own domain, and it carries the
		// documents, committee reports and every step of the business. A link
		// that rots or points at a third party in a year cannot be repaired
		// after the fact; a number that takes one more click can.
		//
		// Both URLs get it, because the reason applies to both.
		//
		// What that reasoning got wrong is the click. The canton does not add a
		// vote to this page until ten to sixteen days after the sitting — the
		// 31.08.2026 votes were still absent on 09.09 — so for the first
		// fortnight the permalink leads to a business matter with no sign of the
		// vote the post is about. Durability was never the whole requirement.
		// So the archive and sitting links ride alongside in their own fields
		// (see VideoURL, SessionURL): the permalink stays first and stays the
		// one that will still resolve in a decade, and the links that have the
		// vote today follow it.
		v.GroupURL = u
		v.SourceURL = u
	}
}

// groupingNumber is a placeholder business number, used until the /affairs call
// supplies the real one.
//
// It must never be empty. Votes are grouped on business number plus sitting
// day, so an empty value makes every such vote of one day look like the same
// business matter: 47 Kanton Zürich votings carry no affair_id at all, eight of
// them on 2025-11-24 alone, and those would be published as one post claiming
// eight unrelated votes belong together.
//
// Without an affair the vote is its own group, so it falls back to its own id.
// sameHeadline reports whether the voting title merely restates the affair
// title, so the subtitle can be dropped.
//
// The comparison cannot be literal. The two strings come from different fields
// filled in by different people, and they disagree on typography: a Richtplan
// item read «Verkehr» as the voting title against "Verkehr" as the affair
// title. Nothing downstream can recover from that — the subtitle survives, and
// every sub-vote in the post reprints the headline it already carries.
func sameHeadline(a, b string) bool {
	return normalizeHeadline(a) == normalizeHeadline(b)
}

// normalizeHeadline strips the differences that are typography rather than
// meaning: quotation marks of every shape, case, and runs of whitespace.
func normalizeHeadline(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lastWasSpace := false
	for _, r := range s {
		switch r {
		case '«', '»', '‹', '›', '"', '„', '“', '”', '\'', '‘', '’':
			continue
		}
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				b.WriteRune(' ')
				lastWasSpace = true
			}
			continue
		}
		lastWasSpace = false
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.TrimSpace(b.String())
}

func groupingNumber(affairID *int64, externalID string) string {
	if affairID != nil {
		return fmt.Sprintf("#%d", *affairID)
	}
	return "#vote-" + externalID
}

// decision returns the source's outcome label, and nothing when the source
// reports none. Kanton Zürich reports one for binary votes only (upstream MR
// !1264); applyDetails gates it by affair and ballot type before it is posted.
//
// An earlier version derived one from the counts when the source left it null.
// That is wrong, and quietly so. Comparing Ja against Nein only decides an
// outcome when the two are the opposing sides of one question, and in a quorum
// vote they are not: Nein is structurally always 0, because there is no Nein to
// cast. Every cantonal quorum vote therefore derived as "Ja" — including one
// with 41 supporters out of 180, published as "✅ Angenommen". Whether such a
// vote passed depends on a threshold that no source we read publishes, and that
// differs by which procedure the vote serves, which type_de does not say.
//
// So we report what the source reports. Downstream, voteformat.HasVerdict
// renders the counts without an outcome line when this is empty. Upstream now
// fills it for binary Kanton Zürich votes, but cannot yet tell an Ausgabenbremse
// from an ordinary one (OpenParlData GitLab #183), so applyDetails clears it
// wherever the ballot or the business does not state its outcome. That guard is
// temporary.
func decision(v votingDTO) string {
	return deref(v.Decision)
}

// choiceLabels maps OpenParlData's harmonised vote values onto the German
// vocabulary the formatters render and the Fraktion table groups by.
//
// Mapping from the harmonised value rather than the display string is what
// makes two bodies' breakdowns line up: Kanton Zürich labels an abstention
// "Enthalten" and Stadt Zürich "Enthaltung", and a table with both columns
// would be wrong rather than merely ugly.
var choiceLabels = map[string]string{
	"yes":        "Ja",
	"no":         "Nein",
	"abstention": "Enthaltung",
	"absent":     "Abwesend",
}

func toMemberVote(v voteDTO) votes.MemberVote {
	choice, ok := choiceLabels[strings.ToLower(v.Vote)]
	if !ok {
		// An unrecognised value still belongs in the table under its own
		// label — dropping the member would understate their faction.
		choice = deref(v.VoteDisplayDe)
		if choice == "" {
			choice = v.Vote
		}
	}

	return votes.MemberVote{
		Name:     deref(v.PersonFullname),
		Party:    deref(v.PersonPartyDe),
		Fraktion: fraktionName(deref(v.PersonParliamentaryGroupNameDe)),
		Choice:   choice,
	}
}

// fraktionName trims the redundant "Fraktion" some bodies include, so faction
// labels read the same across sources.
//
// Both placements occur: Kanton Zürich publishes "Fraktion SVP" throughout its
// vote records, and "SVP-Fraktion" on the odd person record.
func fraktionName(name string) string {
	name = strings.TrimSpace(name)
	if rest, ok := strings.CutPrefix(name, "Fraktion "); ok {
		return strings.TrimSpace(rest)
	}
	if rest, ok := strings.CutSuffix(name, "-Fraktion"); ok {
		return strings.TrimSpace(rest)
	}
	return name
}

// parseDate returns the zero time for anything unparseable; callers treat that
// as "unknown", not "old".
//
// A date with no time of day stays at local midnight. Read as UTC and shifted
// into the body's zone it would become 01:00 or 02:00, and formatters that show
// a vote's time cannot tell that clock reading from a real one.
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		t, err := time.Parse(layout, s)
		if err != nil {
			continue
		}
		if layout == dateOnlyLayout {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, bodyLocation)
		}
		return t.In(bodyLocation)
	}
	return time.Time{}
}

func sequenceFromDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return strconv.FormatInt(t.Unix(), 10)
}
