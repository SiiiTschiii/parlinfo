// Package votes defines the source-neutral vote model that everything
// downstream of data acquisition speaks: formatters, image generation, the
// vote log and the posting pipeline.
//
// It deliberately imports no source package. Adapters (pkg/zurichapi for the
// Stadt Zürich PARIS API, pkg/openparldata for OpenParlData) depend on this
// package, never the other way round.
package votes

import (
	"strings"
	"time"
)

// Jurisdiction identifies one parliamentary body whose votes are posted as a
// unit: its own vote log, its own contacts file and its own age guard.
type Jurisdiction struct {
	// Key is the stable identifier used in file paths and configuration,
	// e.g. "zurich-city". It must be filesystem-safe.
	Key string
	// Name is the human-readable body name, e.g. "Gemeinderat Stadt Zürich".
	Name string
	// ShortName labels posts, e.g. "Gemeinderat". Kept separate from Name so
	// post copy stays short while logs and config stay unambiguous.
	ShortName string
}

// Vote is a single recorded vote in a parliamentary body.
//
// Totals (Yes/No/Abstention/Absent and ChoiceA..E) are first-class fields and
// must never be derived by summing MemberVotes: sources publish the two
// independently and they can legitimately disagree. Formatters read the totals;
// MemberVotes is optional enrichment used for the per-Fraktion breakdown.
type Vote struct {
	// SourceID is the source's stable identifier for this vote and doubles as
	// the dedup key in the vote log (PARIS OBJ_GUID, OpenParlData external_id).
	SourceID string
	// Jurisdiction is the Jurisdiction.Key this vote belongs to.
	Jurisdiction string
	// Body is the chamber's short display name, e.g. "Gemeinderat". It is
	// denormalised onto the vote so formatters can label which body voted
	// without depending on the configuration registry — and label it they must,
	// because two bodies share one account and a reader seeing a single post
	// has nothing else to go on.
	Body string
	// SessionID identifies the sitting this vote was taken in. May be empty
	// when the source has no session concept.
	SessionID string
	// Sequence orders votes within a session. Numeric strings sort numerically.
	// May be empty.
	Sequence string

	// Date is the sitting date. The zero value means the source supplied a date
	// that could not be parsed; callers must treat that as "unknown", not "old".
	Date time.Time

	// Title is the headline text, already resolved between competing source
	// fields (see zurichapi.ToVote). Still needs presentation cleanup.
	Title string
	// Subtitle names the specific question voted on within the Title's business
	// item ("Schlussabstimmung", "Änderungsantrag 9"). Empty when the source has
	// no such concept — Kanton Zürich has no Traktandum.
	Subtitle string
	// Type is the source's vote-type label. Empty when the source omits it.
	Type string

	// SourceURL links to this single vote. GroupURL links to the page covering
	// the whole group when several votes from one Affair are posted together;
	// it falls back to SourceURL when the source has no such page.
	SourceURL string
	GroupURL  string

	// VideoURL links to the recording of the debate, positioned at this vote:
	// Kanton Zürich's Recapp segment, which also names the vote, states the
	// result and offers the Namensliste. VideoGroupURL is the same recording
	// widened to the agenda item, for a post covering several votes.
	//
	// It and ArchiveURL exist because the business page is the durable link
	// but not the prompt one. Kanton Zürich adds a vote to its Geschäft page
	// ten to sixteen days after the sitting, so for the first fortnight the
	// permalink a post carries says nothing about the vote the post is about.
	// The recording has it within the hour.
	//
	// Empty when the source publishes no such page.
	VideoURL      string
	VideoGroupURL string

	// ArchiveURL links to the parliament's written record of this one vote:
	// the Nationalrat's Abstimmungsprotokoll, a PDF with the tally and every
	// member's vote, online within seconds of the vote closing. It has no
	// widened form, so a post covering several votes carries it per vote
	// rather than in the post's own link block.
	//
	// Empty when the source publishes no such record.
	ArchiveURL string

	// SessionURL links to the sitting this vote was taken in, where the body
	// publishes the Traktandenliste, the Bulletin and the recording as one
	// entry. It is per-sitting rather than per-vote, so every vote of a day
	// carries the same value.
	//
	// Empty when the source has no sitting page.
	SessionURL string

	// BulletinURL links to the sitting's Bulletin, the parliament's own
	// summary of what it decided that day. It is filled only where a post
	// needs the reader to look the outcome up themselves, because the source
	// serves the document but not its text — see IsWahlgeschaeft.
	//
	// Empty for every other vote, which is nearly all of them.
	BulletinURL string

	// Totals. Nil means "not reported", which is distinct from zero.
	Yes, No, Abstention, Absent *int
	ChoiceA, ChoiceB, ChoiceC   *int
	ChoiceD, ChoiceE            *int

	// Decision is the source's outcome label ("angenommen", "abgelehnt",
	// "Auswahl A"). Derived from the totals when the source omits it.
	Decision string

	// Attribution is the "Daten: …" line naming the source, closing the link
	// block of every post. Two sources' terms require it (OpenParlData's CC BY
	// 4.0, the Parlamentsdienste's open data terms); the others carry one too,
	// so posts name their data the same way across parliaments. It rides on the
	// vote rather than being looked up because it is a property of where this
	// particular datum came from.
	Attribution string

	Affair      Affair
	MemberVotes []MemberVote
}

// Affair is the business matter (Geschäft) a vote belongs to. Votes are grouped
// for posting by Affair.Number plus sitting date.
type Affair struct {
	Number string
	Title  string
	ID     string
	URL    string

	// Type is the source's name for the kind of business this is ("Vorlage",
	// "Einzelinitiative", "Postulat"). It decides what a Ja actually achieved:
	// a Vorlage that carries is adopted, while an Einzelinitiative that carries
	// is only provisionally supported and goes on to the Regierungsrat. Empty
	// when the source does not say, or when the vote belongs to no business.
	Type string

	// Authors are the members who put this business before the chamber. Empty
	// for business that comes from the government or a committee rather than
	// from members, and for a source that does not publish authorship.
	//
	// It exists because who filed something is half of what a vote on it means,
	// and the two sources supply that fact in incompatible shapes: Stadt
	// Zürich's PARIS writes it into the title ("Postulat von Ivo Bieri (SP)
	// und …"), while OpenParlData serves it as a relation and leaves the title
	// a bare subject line. Formatters read this field; the city's titles are
	// left exactly as they arrive.
	Authors []Author
}

// Author is a member who signed a business matter into the chamber: the
// Erstunterzeichnende first, then the Mitunterzeichnende in the order the
// parliament lists them. Formatters render as many as a post can carry and
// name the rest in their own block, so consumers should expect the whole list
// rather than a lead alone.
//
// Party is required rather than decorative: it is what distinguishes a member
// acting in the chamber from a private person exercising a right of initiative,
// whom the parliament's record names but this project has no business
// broadcasting. See openparldata.affairAuthors.
type Author struct {
	Name  string
	Party string
}

// MemberVote is one member's recorded vote.
//
// Choice uses the German vocabulary the formatters render: "Ja", "Nein",
// "Enthaltung", "Abwesend" for standard votes and "A".."E" for Auswahl votes.
// Adapters normalise into that vocabulary so the breakdown looks the same
// whichever source produced it.
type MemberVote struct {
	Name     string
	Party    string
	Fraktion string
	Choice   string
}

// DateString renders the sitting date as YYYY-MM-DD, or "" when unknown.
func (v Vote) DateString() string {
	if v.Date.IsZero() {
		return ""
	}
	return v.Date.Format("2006-01-02")
}

// AttendanceType is the roll call establishing who is in the chamber. It is
// not a vote on anything, and its lopsided Ja tally must never be published as
// one.
const AttendanceType = "Anwesenheitsermittlung"

// WahlAffairType is the Kantonsrat's own Geschäftsart for an election.
const WahlAffairType = "Wahl"

// IsWahlgeschaeft reports whether v is the roll call taken on an election
// business — the only trace an election leaves in the data.
//
// Both halves are stated by a source, not inferred: the Geschäftsart comes
// from the canton's own business record, and the vote type from the
// parliament's archive. What the combination does *not* say is how the
// election ended. The ballot itself is never recorded: a secret ballot under
// § 125 KRG leaves only this roll call, and an uncontested election under
// § 124 leaves nothing at all. So a post about one can report that the
// business was before the chamber and link the sitting's Bulletin, and must
// claim nothing else — see voteformat.WahlgeschaeftBody.
func IsWahlgeschaeft(v Vote) bool {
	return strings.TrimSpace(v.Affair.Type) == WahlAffairType &&
		strings.TrimSpace(v.Type) == AttendanceType
}

// IsWahlgeschaeftGroup reports whether every vote in a posting group is an
// election roll call, which is what makes the group one Wahlgeschäft notice
// rather than a vote post.
//
// A group can hold several: votes are grouped by business and sitting day, and
// a second ballot round takes a second roll call on the same business. Each of
// them would otherwise take the ordinary multi-vote path and publish its
// headcount as though it were a result.
func IsWahlgeschaeftGroup(group []Vote) bool {
	if len(group) == 0 {
		return false
	}
	for _, v := range group {
		if !IsWahlgeschaeft(v) {
			return false
		}
	}
	return true
}
