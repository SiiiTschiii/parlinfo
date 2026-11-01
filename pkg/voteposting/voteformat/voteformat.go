package voteformat

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

var geschaeftNumberRegex = regexp.MustCompile(`^\d+/\d+\s+`)
var geschaeftNumberUnderscoreRegex = regexp.MustCompile(`^\d+_\d+\s+`)

// FormatVoteDate renders a sitting date as DD.MM.YYYY.
// An unknown (zero) date renders as "" rather than year 1.
func FormatVoteDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02.01.2006")
}

// FormatVoteDateTime renders a vote's timestamp as "DD.MM.YYYY HH:MM", or as
// the date alone where the source has no time of day (see hasClockTime).
func FormatVoteDateTime(t time.Time) string {
	if !hasClockTime(t) {
		return FormatVoteDate(t)
	}
	return t.Format("02.01.2006 15:04")
}

// GetVoteResultEmoji returns the appropriate emoji for a vote result
func GetVoteResultEmoji(result string) string {
	result = strings.TrimSpace(strings.ToLower(result))
	if strings.Contains(result, "angenommen") || result == "ja" {
		return "✅"
	}
	return "❌"
}

// GetVoteResultText returns the text for a vote result
func GetVoteResultText(result string) string {
	result = strings.TrimSpace(strings.ToLower(result))
	if strings.Contains(result, "angenommen") || result == "ja" {
		return "Angenommen"
	}
	return "Abgelehnt"
}

// IsDecisionConsistent reports whether a vote's stated decision is consistent
// with its raw counts. Returns false when the source declares "Ja"/"Angenommen"
// but the Nein count exceeds the Ja count, indicating stale or erroneous data
// that should block posting.
// Auswahl (A/B/C/D) results and nil counts are always considered consistent.
func IsDecisionConsistent(decision string, ja, nein *int) bool {
	if ja == nil || nein == nil {
		return true
	}
	lower := strings.TrimSpace(strings.ToLower(decision))
	if strings.HasPrefix(lower, "auswahl") {
		return true
	}
	statedAccepted := strings.Contains(lower, "angenommen") || lower == "ja"
	return !statedAccepted || *nein <= *ja
}

// GroupPrefixLine returns the label line a post carries above its title: what
// kind of business this is, and — for a lone vote — which question within it was
// put and of what kind.
//
// The business type is worth the words because it changes what the vote means.
// "Keine Baubewilligung mehr für Pergolen in Gärten" reads as a decision taken;
// "Postulat" in front of it reads as what it is, a demand addressed to the
// Regierungsrat. The difference between a government bill, a parliamentary
// demand and an individual initiative is exactly what a reader outside the
// building cannot infer from a tally.
//
// It goes on its own line rather than into the title because the title already
// follows a verdict on single-vote posts: folding it in produced "✅ Angenommen:
// Vorlage: Rahmenkredit …", two colons deep before the subject appears.
//
// It also names who filed the business, when the source says. That is the other
// half of what a vote means, and Stadt Zürich already has it: PARIS writes the
// submitters into the title, so a city post reads "Postulat von Ivo Bieri (SP)
// und …" without this line doing anything. Kanton Zürich serves the same fact
// separately and leaves its titles a bare subject, so the names are put back
// here — which is also what lets the tagger find them, since it matches names
// in the text of a post and cannot tag what the post never says.
//
// Bodies whose source reports no business type are unaffected — Stadt Zürich's
// PARIS adapter never fills Affair.Type, so its posts keep exactly the line they
// had.
func GroupPrefixLine(group []votes.Vote) string {
	if len(group) == 0 {
		return ""
	}

	prefix := strings.TrimSpace(group[0].Affair.Type)

	// Only alongside the type, which is where the "von" attaches: the two
	// arrive from the same call, and "von Marc Bourgeois (FDP)" standing on its
	// own would read as a sentence with its subject missing.
	if prefix != "" {
		if authors := AuthorList(group[0].Affair.Authors, 0); authors != "" {
			prefix += " von " + authors
		}
	}

	// The Abstimmungsgegenstand identifies a question within the business, which
	// is meaningless when several votes share the post; the per-vote headings
	// carry it there instead.
	//
	// The ballot type is deliberately absent from this line. It belongs with the
	// counts, because what it explains is why they read "Zustimmung/ohne"
	// instead of "Ja/Nein" — and a line reading "Vorlage · Ausgabenbremse" put
	// two different kinds of fact side by side as though they were one, the
	// business on the left and the ballot on the right.
	if len(group) == 1 {
		if sub := singleVoteSubtitle(group[0].Subtitle); sub != "" {
			if prefix == "" {
				return sub
			}
			prefix += " · " + sub
		}
	}
	return prefix
}

// AuthorList renders who filed a business as German prose: "Nadia Koch (GLP)",
// or "Marc Bourgeois (FDP) und Tobias Weidmann (SVP)" for several.
//
// The party in brackets is the same shape Stadt Zürich's titles already use, so
// one account posting for two chambers reads as one voice.
//
// max caps how many are named, appending "u. a." for the rest; 0 names all,
// which is what every text post does. The cap exists for the vote card, where
// the type is set in a fixed column and shrinking the font is preferred to
// dropping names — see imagegen.cardPrefix. Cutting the author list is the last
// thing tried there, and never done anywhere else.
func AuthorList(authors []votes.Author, max int) string {
	names := make([]string, 0, len(authors))
	for _, a := range authors {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			continue
		}
		if party := strings.TrimSpace(a.Party); party != "" {
			name += " (" + party + ")"
		}
		names = append(names, name)
	}

	more := ""
	if max > 0 && len(names) > max {
		names, more = names[:max], " u. a."
	}

	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0] + more
	default:
		return strings.Join(names[:len(names)-1], ", ") + " und " + names[len(names)-1] + more
	}
}

// GroupPrefixLineCapped is GroupPrefixLine with at most max authors named.
//
// Only the vote card uses it, and only after the font ladder has run out: the
// subject of the vote is what a reader cannot do without, so it is the last
// thing to give way.
func GroupPrefixLineCapped(group []votes.Vote, max int) string {
	if len(group) == 0 {
		return ""
	}
	capped := make([]votes.Vote, len(group))
	copy(capped, group)
	full := AuthorList(capped[0].Affair.Authors, 0)
	short := AuthorList(capped[0].Affair.Authors, max)
	line := GroupPrefixLine(capped)
	if full == "" || full == short {
		return line
	}
	return strings.Replace(line, "von "+full, "von "+short, 1)
}

// FitAuthorPrefix returns the label line that fits the space a post has left,
// and the signatories it could not fit.
//
// The title is never what gives way. A reader who loses the subject of the vote
// has lost the post; a reader who finds three of five names in the next post of
// the thread has lost nothing. So names are shed from the end until the line
// fits, and the ones shed come back in their own block — see SignatoryLine.
//
// available is the space left for this line after the header, the title and
// whatever the platform appends; measure is the platform's own length function,
// because X counts weighted runes and Bluesky counts graphemes.
func FitAuthorPrefix(group []votes.Vote, available int, measure func(string) int) (string, []votes.Author) {
	if len(group) == 0 {
		return "", nil
	}
	authors := group[0].Affair.Authors

	full := GroupPrefixLine(group)
	if len(authors) == 0 || measure(full) <= available {
		return full, nil
	}

	for n := len(authors) - 1; n >= 1; n-- {
		if line := GroupPrefixLineCapped(group, n); measure(line) <= available {
			return line, authors[n:]
		}
	}

	// Not even one name fits beside this title. The type still labels the
	// business, and every signatory moves to the block below.
	return prefixWithoutAuthors(group), authors
}

// prefixWithoutAuthors is the label line with the authorship removed.
func prefixWithoutAuthors(group []votes.Vote) string {
	bare := make([]votes.Vote, len(group))
	copy(bare, group)
	bare[0].Affair.Authors = nil
	return GroupPrefixLine(bare)
}

// SignatoryLine names the members a post could not fit beside its title.
//
// "Weitere Unterzeichnende" rather than "Mitunterzeichnende" because the split
// is made on length, not on role: the first signatory is named first and is
// almost always the one that fits, but nothing guarantees the boundary falls
// exactly there, and a label that is only usually right is worse than one that
// is always right.
//
// When no name fitted at all, there is nobody for them to be further to, and
// the block carries the label line's own "<type> von …" instead. A federal
// title is often long enough for that: 20.406's leaves no room for Andri
// Silberschmidt, its only author. It says no more about roles than the label
// line would have, which matters where the list holds a member who only took
// the business over: 17.523 is Luzi Stamm's, continued by Bruno Walliser.
func SignatoryLine(group []votes.Vote, deferred []votes.Author) string {
	names := AuthorList(deferred, 0)
	if names == "" {
		return ""
	}
	if len(group) > 0 && len(deferred) == len(group[0].Affair.Authors) {
		if t := strings.TrimSpace(group[0].Affair.Type); t != "" {
			return "✍️ " + t + " von " + names
		}
		return "✍️ Von " + names
	}
	return "✍️ Weitere Unterzeichnende: " + names
}

// CountAuthors reports how many of an affair's signatories a post would name.
func CountAuthors(group []votes.Vote) int {
	if len(group) == 0 {
		return 0
	}
	n := 0
	for _, a := range group[0].Affair.Authors {
		if strings.TrimSpace(a.Name) != "" {
			n++
		}
	}
	return n
}

// CleanVoteTitle removes newlines, extra whitespace, and Geschäft number from titles
func CleanVoteTitle(title string) string {
	// Replace newlines and carriage returns with spaces
	title = strings.ReplaceAll(title, "\r\n", " ")
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.ReplaceAll(title, "\r", " ")

	// Replace multiple spaces with single space
	parts := strings.Fields(title)
	title = strings.Join(parts, " ")

	// Strip Geschäft number from the beginning (e.g., "2024/431 " or "2025/84 ")
	// Pattern: number/number followed by space
	title = geschaeftNumberRegex.ReplaceAllString(title, "")

	return title
}

// CleanVoteSubtitle cleans up vote subtitles (Abstimmungstitel)
// Similar to CleanVoteTitle but keeps it shorter
func CleanVoteSubtitle(subtitle string) string {
	// Replace newlines and carriage returns with spaces
	subtitle = strings.ReplaceAll(subtitle, "\r\n", " ")
	subtitle = strings.ReplaceAll(subtitle, "\n", " ")
	subtitle = strings.ReplaceAll(subtitle, "\r", " ")

	// Replace multiple spaces with single space
	parts := strings.Fields(subtitle)
	subtitle = strings.Join(parts, " ")

	// Strip Geschäft number patterns:
	// Pattern 1: "2025/369 " with slash
	// Pattern 2: "2025_0369 " with underscore
	subtitle = geschaeftNumberRegex.ReplaceAllString(subtitle, "")
	subtitle = geschaeftNumberUnderscoreRegex.ReplaceAllString(subtitle, "")

	return subtitle
}

// FormatVoteCount formats a nullable int pointer
func FormatVoteCount(count *int) string {
	if count == nil {
		return "0"
	}
	return fmt.Sprintf("%d", *count)
}

// VoteCounts holds all possible vote count fields from the API.
// Standard Ja/Nein votes use Ja/Nein/Enthaltung/Abwesend.
// "Gleichgerichtete Anträge mit N Optionen" votes use A/B/C/D/E.
type VoteCounts struct {
	Ja            *int
	Nein          *int
	Enthaltung    *int
	Abwesend      *int
	A, B, C, D, E *int

	// Type is the source's vote-type label, carried alongside the counts
	// because the same four numbers mean different things depending on it: a
	// quorum vote's Nein is structurally 0 and its Abwesend is "did not
	// support", not "was not there".
	Type string
}

// CountsOf reads the totals off a vote. Totals are always taken from the
// source's own count fields and never summed from MemberVotes: sources derive
// the two separately and are allowed to disagree.
func CountsOf(v votes.Vote) VoteCounts {
	return VoteCounts{
		Ja: v.Yes, Nein: v.No, Enthaltung: v.Abstention, Abwesend: v.Absent,
		A: v.ChoiceA, B: v.ChoiceB, C: v.ChoiceC, D: v.ChoiceD, E: v.ChoiceE,
		Type: v.Type,
	}
}

// HasVerdict reports whether a post may state an accepted/rejected outcome for
// this vote. When it is false the formatters print the title and the counts and
// claim nothing about what parliament decided.
//
// Two cases have no verdict to state. An Auswahl vote's outcome is "Auswahl
// A/B/…", not accepted or rejected. And a vote whose source reports no decision
// has none that we are entitled to publish: an outcome is never inferred from
// the counts, because inferring it is only safe when Ja and Nein are the two
// sides of the same question. In a quorum vote they are not — Nein is
// structurally always 0, so every such vote would come out "Angenommen",
// including the ones that failed to reach their threshold. That threshold is
// not published anywhere we can read, and it differs by which procedure the
// vote serves, which type_de does not say. So the honest rendering is silence.
//
// Kanton Zürich reports a decision for binary votes only, and the openparldata
// client clears it again unless the business and the ballot both state their
// outcome, so threshold votes still arrive here without one.
func HasVerdict(v votes.Vote) bool {
	if IsAuswahlVote(CountsOf(v)) {
		return false
	}
	return strings.TrimSpace(v.Decision) != ""
}

// IsAuswahlVote returns true when the vote used the A/B/C/D/E option format
// ("Gleichgerichtete Anträge mit N Optionen") rather than the standard Ja/Nein format.
// In this case result emojis (✅/❌) should be omitted because the outcome is
// "Auswahl A/B/…", not accepted/rejected.
func IsAuswahlVote(c VoteCounts) bool {
	for _, f := range []*int{c.A, c.B, c.C, c.D, c.E} {
		if f != nil && *f > 0 {
			return true
		}
	}
	return false
}

// voteTypeQuorum is the value both sources publish for a quorum vote.
const voteTypeQuorum = "Quorum"

// voteTypeAusgabenbremse is the spending brake, which the Kantonsrat's own
// archive names separately. OpenParlData folds it into "Quorum".
const voteTypeAusgabenbremse = "Ausgabenbremse"

// isThresholdVoteType reports whether a type is decided against a threshold
// rather than by comparing Ja to Nein. Both kinds are counted the same way; they
// differ only in what a post calls them.
func isThresholdVoteType(voteType string) bool {
	switch strings.TrimSpace(voteType) {
	case voteTypeQuorum, voteTypeAusgabenbremse:
		return true
	}
	return false
}

// IsQuorumVote reports whether these counts belong to a threshold vote, which is
// counted and rendered differently everywhere it appears.
func IsQuorumVote(c VoteCounts) bool {
	return isThresholdVoteType(c.Type) && !IsAuswahlVote(c)
}

// formatQuorumCounts renders the summary line for a quorum vote, or "" when the
// vote is not one.
//
// A quorum vote counts supporters against a threshold and has no Nein option,
// so the standard four-part line prints "0 Nein | 0 Enth." for positions nobody
// could have taken, and files every non-supporter under "Abwesend" — which the
// official record shows is wrong for most of them (6 truly absent against 46
// present and not voting, where the API reports 52).
//
// Two honest numbers instead: how many supported, and how many did not. Both
// stay true whether or not the source ever separates absence from abstention,
// and the line is shorter than the standard one, so neither platform needs a
// shortened variant.
func formatQuorumCounts(c VoteCounts) string {
	if !IsQuorumVote(c) {
		return ""
	}
	_, ohne := QuorumTally(c)
	return fmt.Sprintf("📊 %s Zustimmungen | %d ohne Zustimmung",
		FormatVoteCount(c.Ja), ohne)
}

// QuorumTally splits a quorum vote into the two numbers that are true of it:
// how many supported, and how many did not.
//
// Nein is folded into the second rather than read as a third column. It used to
// be 0 on every quorum vote the source served, but that is not a property of
// quorum votes — it was a property of which ones carried a type. The 17.08.2026
// Ausgabenbremse is a binary ballot with a threshold and reports 141 Ja to 1
// Nein, and OpenParlData is separately considering mapping the Kantonsrat's
// "Nicht abgestimmt" onto no instead of absent (upstream #179). Either way both
// buckets mean "did not support", which is exactly what the label says.
//
// It is exported and shared because the caption, the Fraktion table and the
// generated image all state this number, and they must not be able to disagree:
// while every quorum vote had Nein=0 the image's own reading — absent alone —
// looked correct, and it silently stopped being correct the moment one didn't.
func QuorumTally(c VoteCounts) (support, without int) {
	return deref(c.Ja), deref(c.Nein) + deref(c.Abwesend)
}

// deref reads a nullable count, treating "not reported" as zero.
func deref(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// IsUnsupportedVoteType returns true when all active voter count fields are nil or zero.
// This indicates a vote format we don't know how to render (neither standard Ja/Nein
// nor Auswahl A/B/C/D). Callers should log a warning and skip posting such votes.
func IsUnsupportedVoteType(c VoteCounts) bool {
	fields := []*int{c.Ja, c.Nein, c.Enthaltung, c.A, c.B, c.C, c.D, c.E}
	for _, f := range fields {
		if f != nil && *f > 0 {
			return false
		}
	}
	return true
}

// IsEmptyQuorumRound reports whether these counts belong to a quorum round in
// which nobody voted: type "Quorum" exactly (not the Ausgabenbremse, which
// shares its rendering but not its history) and every count nil or zero. A
// negative count is a fault in the source, not an empty round.
func IsEmptyQuorumRound(c VoteCounts) bool {
	if strings.TrimSpace(c.Type) != voteTypeQuorum {
		return false
	}
	for _, f := range []*int{c.Ja, c.Nein, c.Enthaltung, c.A, c.B, c.C, c.D, c.E} {
		if f != nil && *f != 0 {
			return false
		}
	}
	return true
}

// FormatVoteCounts returns the 📊 summary line for a vote.
// Detects Auswahl A/B/C/D votes vs standard Ja/Nein automatically.
// Call IsUnsupportedVoteType first if you need to guard against unknown formats.
func FormatVoteCounts(c VoteCounts) string {
	if line := formatQuorumCounts(c); line != "" {
		return line
	}
	abwesend := FormatVoteCount(c.Abwesend)

	// Check if any Auswahl option has votes
	auswahlPtrs := []*int{c.A, c.B, c.C, c.D, c.E}
	letters := []string{"A", "B", "C", "D", "E"}
	var auswahlParts []string
	for i, f := range auswahlPtrs {
		if f != nil && *f > 0 {
			auswahlParts = append(auswahlParts, fmt.Sprintf("%s: %d", letters[i], *f))
		}
	}
	if len(auswahlParts) > 0 {
		return fmt.Sprintf("📊 %s | Abw. %s", strings.Join(auswahlParts, " | "), abwesend)
	}

	// Standard Ja/Nein vote (short labels for space-constrained platforms)
	return fmt.Sprintf("📊 %s Ja | %s Nein | %s Enth. | %s Abw.",
		FormatVoteCount(c.Ja),
		FormatVoteCount(c.Nein),
		FormatVoteCount(c.Enthaltung),
		abwesend)
}

// FormatVoteCountsLong is like FormatVoteCounts but uses full German label names
// ("Enthaltung", "Abwesend") suited for platforms without a tight character limit.
func FormatVoteCountsLong(c VoteCounts) string {
	if line := formatQuorumCounts(c); line != "" {
		return line
	}
	abwesend := FormatVoteCount(c.Abwesend)

	auswahlPtrs := []*int{c.A, c.B, c.C, c.D, c.E}
	letters := []string{"A", "B", "C", "D", "E"}
	var auswahlParts []string
	for i, f := range auswahlPtrs {
		if f != nil && *f > 0 {
			auswahlParts = append(auswahlParts, fmt.Sprintf("%s: %d", letters[i], *f))
		}
	}
	if len(auswahlParts) > 0 {
		return fmt.Sprintf("📊 %s | Abwesend %s", strings.Join(auswahlParts, " | "), abwesend)
	}

	return fmt.Sprintf("📊 %s Ja | %s Nein | %s Enthaltung | %s Abwesend",
		FormatVoteCount(c.Ja),
		FormatVoteCount(c.Nein),
		FormatVoteCount(c.Enthaltung),
		abwesend)
}

// IsSchlussabstimmung returns true if the Abstimmungstitel contains "Schlussabstimmung" (case-insensitive).
func IsSchlussabstimmung(abstimmungstitel string) bool {
	return strings.Contains(strings.ToLower(abstimmungstitel), "schlussabstimmung")
}

func singleVoteSubtitle(abstimmungstitel string) string {
	if abstimmungstitel == "" {
		return ""
	}
	if IsSchlussabstimmung(abstimmungstitel) {
		return ""
	}
	return CleanVoteSubtitle(abstimmungstitel)
}

// typeLabels names the vote types worth surfacing to a reader, keyed by the
// value the sources publish. PARIS (Abstimmungstyp) and OpenParlData (type_de)
// share this vocabulary, so one table serves both bodies.
//
// It is an allowlist rather than a passthrough because most of the vocabulary
// is either uninformative or already visible in the counts: "Normal" describes
// the overwhelming majority and says nothing, and "Gleichgerichtete Anträge mit
// 4 Optionen" restates an Auswahl vote that already renders as A/B/C/D. Only
// types whose tally would otherwise be read wrongly earn a label.
var typeLabels = map[string]string{
	// A quorum vote counts supporters against a threshold and has no Nein
	// option at all, so everyone not supporting it lands in Abwesend. Unlabelled,
	// "129 Ja | 0 Nein | 51 Abw." reads as near-unanimous agreement rather than
	// as a procedural vote most of the opposition deliberately sits out.
	"Quorum": "Quorum",
	// The spending brake, which needs 91 of 180 votes whatever the opposition
	// does. It is counted like any other threshold vote but named, because
	// "Ausgabenbremse" tells a reader what the threshold was for where "Quorum"
	// only tells them that one existed.
	"Ausgabenbremse": "Ausgabenbremse",
	// A knockout round between more than two competing proposals. Kanton Zürich
	// reports these with no aggregate counts at all, so they are rejected before
	// posting today (see IsUnsupportedVoteType); the label is here so they read
	// correctly if that is ever fixed upstream.
	"Cup-Abstimmung": "Cup-Abstimmung",
}

// TypeLabel returns the reader-facing label for a vote type, or "" when the
// type does not warrant one.
func TypeLabel(voteType string) string {
	return typeLabels[strings.TrimSpace(voteType)]
}

// auswahlTypePrefix begins the type both sources use for multi-option votes,
// which carry an option count in their name ("… mit 3 Optionen", "… mit 4
// Optionen"), so the whole family has to be matched by prefix.
const auswahlTypePrefix = "Gleichgerichtete Anträge"

// handledVoteTypes are the types whose counts the formatters render correctly.
// It is deliberately narrower than typeLabels: rendering a type correctly and
// having a name for it are different things.
var handledVoteTypes = map[string]bool{
	// Ja/Nein/Enthaltung/Abwesend, the overwhelming majority.
	"Normal": true,
	// Supporters counted against a threshold, no Nein option.
	"Quorum": true,
	// A binary ballot that carries only on reaching 91 of 180 votes.
	"Ausgabenbremse": true,
}

// unpostableVoteTypes are types we recognise and have decided not to publish.
//
// They are named separately from the types we have simply never seen, because
// the two call for opposite responses. A type on this list is expected to turn
// up — every Kanton Zürich sitting opens with an attendance roll call — and
// skipping it is the system working, so it must not fail a run. An unrecognised
// type is the source doing something new, which is exactly what someone needs
// to be told about.
// Cup-Abstimmung is deliberately absent. It is also never published, but for a
// reason that is expected to go away: the source serves it with null aggregates
// and duplicated member rows, both reported upstream. Its run-failing rejection
// is the reminder that the report is still open, so it stays an
// ErrUnsupportedVoteType — see TestPostToPlatform_CantonCupVoteIsNotPosted.
var unpostableVoteTypes = map[string]bool{
	// A roll call establishing who is in the chamber. It is not a vote on
	// anything and reports as a lopsided Ja tally, so publishing one would
	// announce a near-unanimous decision on a question nobody was asked.
	// Nothing upstream is broken here and nothing is going to change: the
	// Kantonsrat takes one at the start of most sittings.
	"Anwesenheitsermittlung": true,
}

// IsKnownUnpostableType reports whether a type is one we recognise and
// deliberately do not publish, as opposed to one we do not recognise at all.
func IsKnownUnpostableType(voteType string) bool {
	return unpostableVoteTypes[strings.TrimSpace(voteType)]
}

// WahlgeschaeftNoResult is the one statement a Wahlgeschäft post makes. It is
// about this project's data, not about the chamber, because the chamber's
// decision is the part nothing here can see: the ballot is not recorded, so
// neither the outcome nor the person elected is available to assert. See
// votes.IsWahlgeschaeft.
const WahlgeschaeftNoResult = "Zu dieser Wahl liegen uns keine Ergebnisdaten vor."

// WahlgeschaeftBody renders the reader-facing text of a Wahlgeschäft post: the
// business title exactly as the parliament writes it, then the sentence above.
//
// The title is published verbatim, including its "für <Name>" tail. That tail
// names the member being replaced rather than the one elected, which reads the
// wrong way round to anyone who does not know the convention — but the wording
// is the canton's, the post makes no claim about it, and every alternative
// tried so far meant this project deciding which half of an official title to
// print. A misleading sentence quoted intact is a fault in the source that a
// reader can check against the linked Bulletin; the same sentence edited by us
// is a fault in the post.
func WahlgeschaeftBody(group []votes.Vote) string {
	if len(group) == 0 {
		return WahlgeschaeftNoResult
	}
	title := CleanVoteTitle(group[0].Title)
	if title == "" {
		return WahlgeschaeftNoResult
	}
	return title + "\n\n" + WahlgeschaeftNoResult
}

// IsHandledVoteType reports whether the formatters know how to render a vote of
// this type. Callers skip anything else rather than publish it.
//
// This is an allowlist, and an empty type does not pass it. That is the point:
// a type we have never seen is exactly the case where a lopsided tally is most
// likely to be read wrongly, and refusing to post is recoverable in a way that
// a misleading post about how parliament voted is not.
//
// OpenParlData serves a null type often enough that this used to reject whole
// Kanton Zürich sittings — the 17.08.2026 sitting arrived with all five of its
// votes untyped, among them two Ausgabenbremse votes and an attendance roll
// call. Those votes now arrive typed from the Kantonsrat's own archive (see
// pkg/recapp), so an empty type here means both sources came up blank, which is
// exactly when refusing is right.
//
// Cup-Abstimmung is knowingly excluded. It is a knockout round between more
// than two proposals, and the source reports no aggregate counts for it, so
// there is nothing correct to render yet.
func IsHandledVoteType(voteType string) bool {
	t := strings.TrimSpace(voteType)
	if handledVoteTypes[t] {
		return true
	}
	return strings.HasPrefix(t, auswahlTypePrefix)
}

// joinTypeLabel appends the type label to base with a separator, handling the
// case where base is empty and the label has to stand alone.
func joinTypeLabel(base, voteType string) string {
	label := TypeLabel(voteType)
	switch {
	case label == "":
		return base
	case base == "":
		return label
	default:
		return base + " · " + label
	}
}

// GroupLink returns the URL a post about this group should point at: the page
// covering the whole group when several votes are posted together, otherwise
// the single vote's own page. Sources decide what those two URLs are; this only
// picks between them.
func GroupLink(group []votes.Vote) string {
	if len(group) == 0 {
		return ""
	}
	if len(group) > 1 && group[0].GroupURL != "" {
		return group[0].GroupURL
	}
	return group[0].SourceURL
}

// BodyLabel returns the chamber name to put in a post header.
//
// Several bodies post to one account, so this is what tells a reader whether
// they are looking at a city or a cantonal vote. It is not decoration.
func BodyLabel(group []votes.Vote) string {
	if len(group) > 0 && group[0].Body != "" {
		return group[0].Body
	}
	return ""
}

// PostHeadline is the first line of a post: which chamber voted, and when.
//
// A single-vote post names the time as well, where the source knows it. The
// post is about one moment, and without it two votes taken under the same
// agenda item on the same day — "Mitteilungen" is the recurring case, and it
// carries no business number, so each such vote posts on its own — produce two
// posts a reader cannot tell apart. Multi-vote groups span several times and
// keep the date alone; their entries carry the clock (see SubVoteLabel).
//
// The date clause is dropped when the date is unknown. Votes with an
// unparseable date are deliberately kept rather than discarded — silently
// dropping a vote is worse than posting one whose date we could not read — so
// this case is reachable, and "Abstimmung vom " trailing into nothing is not an
// acceptable way to render it.
//
// Callers prepend their own emoji, because the platforms do not agree on the
// spacing after it and that difference is already baked into published posts.
func PostHeadline(group []votes.Vote) string {
	body := BodyLabel(group)
	prefix := ""
	if body != "" {
		prefix = body + " | "
	}
	if len(group) == 0 || group[0].Date.IsZero() {
		if votes.IsWahlgeschaeftGroup(group) {
			return prefix + "Wahlgeschäft"
		}
		return prefix + "Abstimmung"
	}
	// A Wahlgeschäft post reports no vote, so it is not labelled as one, and it
	// carries no clock time: the only timestamp available is the roll call's,
	// and printing it would date an election to the minute the chamber was
	// counted.
	if votes.IsWahlgeschaeftGroup(group) {
		return prefix + "Wahlgeschäft vom " + FormatVoteDate(group[0].Date)
	}

	headline := prefix + "Abstimmung vom " + FormatVoteDate(group[0].Date)
	if len(group) == 1 {
		headline += clockSuffix(group[0].Date)
	}
	return headline
}

// Link is one entry of a post's trailing link block: where it goes, and what to
// call it when the post carries more than one.
type Link struct {
	// Icon prefixes the URL on every post.
	Icon string
	// Name is the German noun for what the link opens. It is rendered only
	// where a post carries several links, because that is the only place it
	// earns its characters: three bare URLs one under the other tell a reader
	// nothing about which to open, while a lone link needs no disambiguating
	// and every published post so far has rendered it without one.
	Name string
	URL  string
}

// Line renders one entry. labelled says whether the post carries several links.
func (l Link) Line(labelled bool) string {
	if labelled && l.Name != "" {
		return l.Icon + " " + l.Name + ": " + l.URL
	}
	return l.Icon + " " + l.URL
}

// GroupLinks returns every link a post about this group carries, in the order
// LinkLine renders them.
//
// The order is the argument. The Geschäft permalink comes first because it is
// the one that will still resolve in a decade and the one a reader should keep;
// the record of the vote, the recording and the sitting follow because they are
// the ones that have the vote this week. Sources that publish only a business
// page produce a single-entry block, exactly as before.
//
// The video entry follows GroupLink's rule: a post covering several votes
// points at the agenda item holding them, not at whichever of them happened to
// sort first. The Protokoll has no such widened form, so it appears only on a
// post about one vote; VoteLinks carries it for each vote of a larger group.
func GroupLinks(group []votes.Vote) []Link {
	if len(group) == 0 {
		return nil
	}
	first := group[0]

	video := first.VideoURL
	protocol := first.ArchiveURL
	if len(group) > 1 {
		// No widened link means no entry rather than an arbitrary one.
		video = first.VideoGroupURL
		protocol = ""
	}

	// A vote that belongs to no business matter never had its URLs replaced by
	// an affair page, so both fields still hold the archive link the listing
	// supplied — 47 Kanton Zürich votings carry no affair_id at all. Printing it
	// twice would label the first copy "Geschäft", which it is not, and give
	// Bluesky two facets hunting the same byte offset. The video naming is the
	// honest one, so that is the copy that survives.
	business := GroupLink(group)
	if business == video {
		business = ""
	}

	// The Bulletin stands in for the sitting page where one was fetched, rather
	// than joining it: it is the same sitting, one click deeper, and it is the
	// only reason a Wahlgeschäft post exists at all. Every other vote leaves
	// the field empty and keeps the sitting link.
	sitting := Link{Icon: "📄", Name: "Sitzung", URL: first.SessionURL}
	if first.BulletinURL != "" {
		sitting = Link{Icon: "📄", Name: "Bulletin", URL: first.BulletinURL}
	}

	var out []Link
	for _, l := range []Link{
		{Icon: "🔗", Name: "Geschäft", URL: business},
		{Icon: "📋", Name: "Protokoll", URL: protocol},
		{Icon: "🎬", Name: "Video", URL: video},
		sitting,
	} {
		if l.URL != "" {
			out = append(out, l)
		}
	}
	return out
}

// LinkLine returns the trailing block of a post: the links, plus the source
// credit when the data's licence requires one.
//
// They are built together so that the platforms' character-budget arithmetic
// accounts for the credit. Appending it afterwards would make posts overflow
// exactly on the votes that need it.
func LinkLine(group []votes.Vote) string {
	links := GroupLinks(group)
	if len(links) == 0 {
		return ""
	}
	out := "\n"
	labelled := len(links) > 1
	for _, l := range links {
		out += "\n" + l.Line(labelled)
	}
	if group[0].Attribution != "" {
		out += "\n" + group[0].Attribution
	}
	return out
}

// SubVoteLabel names one vote inside a group, so the entries of a thread or the
// cards of a carousel can be told apart. groupSize is how many votes the group
// holds; a group of one needs no such label beyond whatever title it has.
//
// Stadt Zürich titles each vote ("Schlussabstimmung über die Dispositivziffer
// 1"). Kanton Zürich publishes no per-vote title at all — its title field
// repeats the business matter — so those groups fall back to an ordinal plus
// the time the vote was taken: "Abstimmung 2 (14:32)".
//
// The time was tried once before and removed, on the grounds that the link
// already lands in the right place and a clock reading means no more to a
// reader than a number does. What that judgement did not account for is how the
// ordinals look in practice: five cards reading "Abstimmung 1" to "Abstimmung 5"
// above near-identical tallies give a reader nothing to hold on to, and no way
// to tell whether the near-identical numbers are five votes or one repeated.
// The clock is thin, but it is a fact about the sitting rather than an artefact
// of our own numbering, and it is the only distinguishing fact the source
// offers. It appears only where it can help: multi-vote groups from a source
// with second-precision timestamps.
//
// The vote type is appended after all of that, when it is one worth naming (see
// typeLabels). Unlike the clock it is not there to tell two votes apart, but to
// stop a quorum vote's lopsided tally reading as near-unanimous agreement.
func SubVoteLabel(v votes.Vote, index, groupSize int) string {
	return subVoteLabel(v, index, groupSize, true)
}

// CardSubVoteLabel is SubVoteLabel without the clock, for a result card that
// prints the vote's full date and time in its corner instead. Splitting the
// two — date in the corner, time in the heading — left a reader to put the
// timestamp back together.
func CardSubVoteLabel(v votes.Vote, index, groupSize int) string {
	return subVoteLabel(v, index, groupSize, false)
}

func subVoteLabel(v votes.Vote, index, groupSize int, withClock bool) string {
	base := CleanVoteSubtitle(v.Subtitle)
	if base == "" {
		base = fmt.Sprintf("Abstimmung %d", index+1)
		// The clock disambiguates our own ordinals, so it belongs only on them.
		// A source that titles its votes has already done the distinguishing.
		if withClock && groupSize > 1 {
			base += clockSuffix(v.Date)
		}
	}
	return joinTypeLabel(base, v.Type)
}

// clockSuffix is the " (14:32)" a text label appends for the time a vote was
// taken, or "" where the source has no time of day.
func clockSuffix(t time.Time) string {
	if !hasClockTime(t) {
		return ""
	}
	return " (" + t.Format("15:04") + ")"
}

// hasClockTime reports whether a date carries a time of day worth showing.
// PARIS supplies sitting dates at midnight, so printing "(00:00)" there would
// invent a precision the source does not have.
func hasClockTime(t time.Time) bool {
	return !t.IsZero() && (t.Hour() != 0 || t.Minute() != 0)
}

// VoteLinks returns the links that belong to one vote of a group and not to the
// group as a whole, for a platform that shows each vote beside its own card.
//
// A post covering several votes links the business and the agenda item, because
// pointing at whichever vote sorted first would be arbitrary. That leaves each
// vote's own pages unlinked: the recording positioned at this vote (Kanton
// Zürich's Recapp segment), the record of this vote (the Nationalrat's
// Abstimmungsprotokoll) and this vote's own page (Stadt Zürich's Abstimmung,
// beside the Traktandum the post links). All are returned, each only where the
// source has one and only where the post's own link block does not already
// carry the same URL — for Kanton Zürich and the Nationalrat the vote's page is
// the business page, which the post links already.
func VoteLinks(group []votes.Vote, i int) []Link {
	if i < 0 || i >= len(group) {
		return nil
	}
	v := group[i]

	have := map[string]bool{}
	for _, l := range GroupLinks(group) {
		have[l.URL] = true
	}

	var out []Link
	for _, l := range []Link{
		{Icon: "🎬", Name: "Video", URL: v.VideoURL},
		{Icon: "📋", Name: "Protokoll", URL: v.ArchiveURL},
		{Icon: "🔗", Name: "Abstimmung", URL: v.SourceURL},
	} {
		if l.URL == "" || have[l.URL] {
			continue
		}
		have[l.URL] = true
		out = append(out, l)
	}
	return out
}
