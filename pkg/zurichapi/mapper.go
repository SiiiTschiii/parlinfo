package zurichapi

import (
	"strings"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// JurisdictionKey identifies the Gemeinderat der Stadt Zürich, the only body
// PARIS serves.
const JurisdictionKey = "zurich-city"

// Attribution names the source. Open Data Zürich publishes under CC0, which asks
// for no credit; the line is there so every post names its data the same way,
// whichever parliament it comes from.
const Attribution = "Daten: Stadt Zürich"

// Jurisdiction describes the body behind this API.
var Jurisdiction = votes.Jurisdiction{
	Key:       JurisdictionKey,
	Name:      "Gemeinderat Stadt Zürich",
	ShortName: "Gemeinderat ZH",
}

// ToVote converts a PARIS Abstimmung into the source-neutral model.
//
// Two decisions that used to live in the formatters happen here, because they
// are facts about *this source's* fields rather than about presentation:
//
//   - Title resolution. PARIS supplies both a Traktandum title and a Geschäft
//     title, and the Traktandum one is frequently the useless "Antrag 3.".
//     SelectBestTitle picks between them.
//   - Link selection. The generic-Antrag case links to the Geschäft page
//     because the agenda item says nothing; otherwise a single vote links to
//     the vote detail page and a group links to the agenda item covering it.
func ToVote(a Abstimmung) votes.Vote {
	genericAntrag := IsGenericAntragTitle(a.TraktandumTitel)

	sourceURL := VoteLink(a.OBJGUID)
	groupURL := TraktandumLink(a.SitzungGuid, a.TraktandumGuid)
	geschaeftURL := GeschaeftLink(a.GeschaeftGuid)
	if genericAntrag {
		sourceURL = geschaeftURL
		groupURL = geschaeftURL
	}

	return votes.Vote{
		SourceID:     a.OBJGUID,
		Jurisdiction: JurisdictionKey,
		Body:         Jurisdiction.ShortName,
		SessionID:    a.SitzungGuid,
		Sequence:     a.SEQ,
		Date:         parseSitzungDatum(a.SitzungDatum),

		Title:    SelectBestTitle(a.TraktandumTitel, a.GeschaeftTitel),
		Subtitle: a.Abstimmungstitel,
		Type:     a.Abstimmungstyp,

		SourceURL:   sourceURL,
		GroupURL:    groupURL,
		Attribution: Attribution,

		Yes:        a.AnzahlJa,
		No:         a.AnzahlNein,
		Abstention: a.AnzahlEnthaltung,
		Absent:     a.AnzahlAbwesend,
		ChoiceA:    a.AnzahlA,
		ChoiceB:    a.AnzahlB,
		ChoiceC:    a.AnzahlC,
		ChoiceD:    a.AnzahlD,
		ChoiceE:    a.AnzahlE,

		Decision: a.Schlussresultat,

		Affair: votes.Affair{
			Number: a.GeschaeftGrNr,
			Title:  a.GeschaeftTitel,
			ID:     a.GeschaeftGuid,
			URL:    geschaeftURL,
		},

		MemberVotes: ToMemberVotes(a.Stimmabgaben.Stimmabgabe),
	}
}

// ToVotes maps a slice of Abstimmungen.
func ToVotes(as []Abstimmung) []votes.Vote {
	out := make([]votes.Vote, len(as))
	for i, a := range as {
		out[i] = ToVote(a)
	}
	return out
}

// ToMemberVotes maps PARIS Stimmabgaben to the neutral member-vote model.
// PARIS already uses the German vocabulary the formatters render ("Ja",
// "Nein", "Enthaltung", "Abwesend", "A".."E"), so Choice passes through.
func ToMemberVotes(sa []Stimmabgabe) []votes.MemberVote {
	if len(sa) == 0 {
		return nil
	}
	out := make([]votes.MemberVote, len(sa))
	for i, s := range sa {
		out[i] = votes.MemberVote{
			Name:     strings.TrimSpace(s.Vorname + " " + s.Name),
			Party:    s.Partei,
			Fraktion: s.Fraktion,
			Choice:   s.Abstimmungsverhalten,
		}
	}
	return out
}

// sitzungDatumLayouts are the formats PARIS has been observed to use.
var sitzungDatumLayouts = []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"}

// parseSitzungDatum returns the zero time when the date cannot be parsed.
// Callers must treat that as "unknown" — in particular the age guard keeps such
// votes rather than discarding them as old.
func parseSitzungDatum(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range sitzungDatumLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t
		}
	}
	return time.Time{}
}
