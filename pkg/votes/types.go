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
	// Language is what the body's posts are written in, and what its source is
	// asked to serve. The zero value is German.
	Language Language
}

// Language is a post language, as the lower-case code parlament.ch puts in its
// URLs.
//
// A body posting in two languages is two jurisdictions, not one with a
// setting, because what makes a jurisdiction is its vote log: a vote posted
// to the German account has not been posted to the French one.
type Language string

// The post languages. Romansh and English are absent on purpose: parlament.ch
// accepts both codes but serves the German and the French text under them.
const (
	German  Language = "de"
	French  Language = "fr"
	Italian Language = "it"
)

// OrGerman returns l, or German for the zero value.
func (l Language) OrGerman() Language {
	if l == "" {
		return German
	}
	return l
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
	// Language is the jurisdiction's post language, denormalised for the same
	// reason as Body: formatters render from the vote alone.
	Language Language
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

	// Outcome is what the vote did to the business, when that differs in kind
	// from whether Ja carried: "angenommen" or "abgelehnt" for a motion or
	// postulate, "Folge gegeben" or "keine Folge gegeben" for an initiative.
	// Posts state it in place of Decision. Empty for every other vote.
	//
	// It exists because on such a vote Ja is whatever the motion before the
	// chamber proposed, often the committee's proposal to reject. A Ja carrying
	// then means the business failed, and "✅ Angenommen" would say the
	// opposite of what happened. Decision keeps meaning "Ja carried", which is
	// what the consistency gate checks against the counts.
	Outcome string

	// Question says what a Ja vote supported, as a post prints it below the
	// title: "Ja: keine Folge geben". Set where the verdict alone would read as
	// a contradiction, a ✕ above 128 Ja, because Ja was a proposal to reject.
	// Empty for every other vote.
	Question string

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

	// Fraktion names the parliamentary group that filed this business, when a
	// group and not a member did, written so it reads after "der" and names its
	// parties: "SVP-Fraktion", "Mitte-EVP-Fraktion". Empty otherwise.
	//
	// parlament.ch records a Fraktion filing with the group as the filer and
	// no member, so Authors stays empty for it. Zürich's sources leave this
	// empty: the Gemeinderat writes the Fraktion into the title itself, and
	// the tagger finds it there.
	Fraktion string

	// Canton names the canton that filed this business, a Standesinitiative,
	// as a post names it after "des Kantons": "Thurgau". Empty otherwise.
	Canton string
}

// Author is a member who signed a business matter into the chamber: the
// Erstunterzeichnende first, then the Mitunterzeichnende in the order the
// parliament lists them, each with their Role where the source says.
// Formatters render as many as a post can carry and name the rest in their own
// block, so consumers should expect the whole list rather than a lead alone.
//
// Party is required rather than decorative: it is what distinguishes a member
// acting in the chamber from a private person exercising a right of initiative,
// whom the parliament's record names but this project has no business
// broadcasting. See openparldata.affairAuthors.
type Author struct {
	Name  string
	Party string
	Role  AuthorRole
}

// AuthorRole is the part a member had in a business, where the source says.
//
// The zero value is what Stadt and Kanton Zürich give: a signatory, without
// telling the one who filed from the ones who signed with them. parlament.ch
// does tell, and the difference decides where a name goes: the label line
// names who filed and who took the business over, while co-signers — up to
// twenty on a federal initiative — would bury the subject of the vote there,
// and are named only in the text below it.
type AuthorRole int

const (
	// RoleSigned filed or signed the business; the label line names them.
	RoleSigned AuthorRole = iota
	// RoleTookOver continued a business whose filer left the chamber.
	RoleTookOver
	// RoleCoSigned signed beside the filer. Named in the text posts, where a
	// handle can be tagged, never on the label line or the card.
	RoleCoSigned
)

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
