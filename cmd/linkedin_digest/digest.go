package main

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// postedIDs collects every vote any of the logs records as posted.
//
// Any platform counts. The digest is about what the bot published, and a vote
// that reached one account and failed on another was still published. When it
// was posted does not matter: the window is the sitting date, see window.
func postedIDs(logs []*votelog.VoteLog) map[string]bool {
	ids := map[string]bool{}
	for _, l := range logs {
		for _, e := range l.Votes {
			ids[e.ID] = true
		}
	}
	return ids
}

// window is the sitting dates a digest covers, as YYYY-MM-DD: From up to and
// including To.
//
// It is the date the parliament voted, not the date the bot posted. The bot
// drains a sitting over several hourly runs and posts one vote to each platform
// at a different time, so a window on posting dates split sittings and listed
// a vote in two consecutive digests. A sitting date puts each vote in exactly
// one, and the digest's heading then means what a reader takes it to mean.
type window struct{ From, To string }

// newWindow is the days days ending on last.
func newWindow(last time.Time, days int) window {
	return window{
		From: last.AddDate(0, 0, -(days - 1)).Format("2006-01-02"),
		To:   last.Format("2006-01-02"),
	}
}

// holds reports whether v was voted in the window. A vote whose date the
// source did not give is in no window.
func (w window) holds(v votes.Vote) bool {
	d := v.DateString()
	return d != "" && d >= w.From && d <= w.To
}

// keep narrows groups to the votes keep accepts, dropping a group left empty.
//
// It runs after grouping because grouping can pull in votes that were never
// fetched: PARIS completes a Geschäft with its earlier agenda items, and one of
// those the bot skipped must not appear in a digest of what it posted.
func keep(groups [][]votes.Vote, accept func(votes.Vote) bool) [][]votes.Vote {
	var out [][]votes.Vote
	for _, g := range groups {
		var kept []votes.Vote
		for _, v := range g {
			if accept(v) {
				kept = append(kept, v)
			}
		}
		if len(kept) > 0 {
			out = append(out, kept)
		}
	}
	return out
}

// reachesBack reports whether the fetched votes go back past the window's
// start. If none does, the fetch may have stopped inside the window and left
// its first days out.
func reachesBack(fetched []votes.Vote, w window) bool {
	for _, v := range fetched {
		if d := v.DateString(); d != "" && d < w.From {
			return true
		}
	}
	return false
}

// section is one body's part of the digest.
type section struct {
	Name   string
	Groups [][]votes.Vote
}

// maxPostChars is the most characters LinkedIn takes in a post.
const maxPostChars = 3000

// shortestSubject is how far a subject may be cut to make a post fit. Below it
// an entry no longer says what the vote was about, and a post that still does
// not fit is printed as it is, with a warning.
const shortestSubject = 60

// sitting is one body's votes on one day, or with -merge on every day of the
// window it sat: what one LinkedIn post covers.
type sitting struct {
	Body string
	Date time.Time
	// Last is the last day of a merged sitting, zero when it is one day.
	Last   time.Time
	Groups [][]votes.Vote
}

// heading names the body and when it sat: "Kantonsrat Zürich | Sitzung vom
// 09.11.2026", "Nationalrat | Herbstsession 2026, Sitzungen vom 28.09.2026
// bis 02.10.2026", or the body alone where the source gave no date.
func (s sitting) heading() string {
	if s.Date.IsZero() {
		return s.Body
	}
	when := "Sitzung vom " + voteformat.FormatVoteDate(s.Date)
	if !s.Last.IsZero() && !sameDay(s.Date, s.Last) {
		when = "Sitzungen vom " + voteformat.FormatVoteDate(s.Date) + " bis " + voteformat.FormatVoteDate(s.Last)
	}
	if len(s.Groups) > 0 {
		if name := session(s.Groups[0][0]); name != "" {
			when = name + ", " + when
		}
	}
	return s.Body + " | " + when
}

// session names the session of the Federal Assembly a Nationalrat vote was
// taken in, and is empty for any other parliament. The source gives only a
// numeric ID, but the sessions keep to fixed seasons: spring from late
// February, a special session in early May, summer from late May, autumn into
// early October and winter from late November.
func session(v votes.Vote) string {
	if !strings.HasPrefix(v.Jurisdiction, config.NationalratKey) || v.Date.IsZero() {
		return ""
	}
	var name string
	switch m := v.Date.Month(); {
	case m == time.February || m == time.March:
		name = "Frühjahrssession"
	case m == time.April || m == time.May && v.Date.Day() < 20:
		name = "Sondersession"
	case m == time.May || m == time.June:
		name = "Sommersession"
	case m == time.September || m == time.October:
		name = "Herbstsession"
	case m == time.November || m == time.December:
		name = "Wintersession"
	default:
		return ""
	}
	return fmt.Sprintf("%s %d", name, v.Date.Year())
}

func sameDay(a, b time.Time) bool { return a.Format("2006-01-02") == b.Format("2006-01-02") }

// merged folds the sittings of each body into one, from its first day to its
// last: a parliament that sits in sessions, like the Nationalrat, reads better
// as a session than as a post a day.
//
// A business voted on more than one day is one business in the merged post,
// its votes in date order, so it is counted once and highlighted at most once;
// a source groups by business and day, so the days' groups are joined here.
func merged(ss []sitting) []sitting {
	var out []sitting
	at := map[string]int{}
	business := map[string]map[string]int{} // body → business → index in Groups
	for _, s := range ss {
		i, ok := at[s.Body]
		if !ok {
			i = len(out)
			at[s.Body] = i
			business[s.Body] = map[string]int{}
			out = append(out, sitting{Body: s.Body, Date: s.Date, Last: s.Date})
		}
		m := &out[i]
		for _, g := range s.Groups {
			key := businessKey(g)
			if j, seen := business[s.Body][key]; seen && key != "" {
				m.Groups[j] = append(append([]votes.Vote(nil), m.Groups[j]...), g...)
				continue
			}
			if key != "" {
				business[s.Body][key] = len(m.Groups)
			}
			m.Groups = append(m.Groups, g)
		}
		if s.Date.After(m.Last) {
			m.Last = s.Date
		}
	}
	return out
}

// businessKey identifies a group's business, by the source's ID or else its
// number, and is empty for votes that belong to no business.
func businessKey(g []votes.Vote) string {
	if len(g) == 0 {
		return ""
	}
	a := g[0].Affair
	if a.ID != "" {
		return g[0].Jurisdiction + "/id/" + a.ID
	}
	if a.Number != "" {
		return g[0].Jurisdiction + "/nr/" + a.Number
	}
	return ""
}

// sittings splits the sections into one sitting per body and day, in the
// sections' order and by date within each.
func sittings(sections []section) []sitting {
	var out []sitting
	for _, s := range sections {
		for _, g := range bySitting(s.Groups) {
			if n := len(out); n > 0 && out[n-1].Body == s.Name && out[n-1].Date.Format("2006-01-02") == g[0].Date.Format("2006-01-02") {
				out[n-1].Groups = append(out[n-1].Groups, g)
				continue
			}
			out = append(out, sitting{Body: s.Name, Date: g[0].Date, Groups: [][]votes.Vote{g}})
		}
	}
	return out
}

// maxListed is the most businesses a post lists. A sitting with more gets its
// highlights instead: its rejections, then its closest votes.
const maxListed = 5

// options shape a draft.
type options struct {
	// Site is the parlinfo.ch page listing the accounts that post every vote,
	// which is the first comment's one link. Without one the comment links
	// each entry's official record instead.
	Site string
	// All lists every business of a sitting, however many there are.
	All bool
	// Merge makes one post per body of all its sittings in the window.
	Merge bool
}

// posts are the sittings the draft has a post for, in order.
func posts(sections []section, o options) []sitting {
	ss := sittings(sections)
	if o.Merge {
		return merged(ss)
	}
	return ss
}

// render writes the draft: one post per sitting, each with its first comment
// and its tagging list. It also returns how many entries it shortened to make
// a post fit.
//
// A post per sitting, because a week of one parliament does not fit in one:
// the Gemeinderat's sitting of 28 October alone came to 3,595 characters
// against LinkedIn's 3,000.
//
// mapper may be nil, which leaves the tagging lists empty.
func render(sections []section, mapper *contacts.Mapper, o options) (string, int) {
	all := posts(sections, o)
	var out strings.Builder
	shortened := 0
	for i, s := range all {
		draft, cut := renderSitting(s, mapper, o, maxPostChars)
		shortened += cut
		if len(all) > 1 {
			if i > 0 {
				out.WriteString("\n")
			}
			fmt.Fprintf(&out, "══════ Beitrag %d von %d ══════\n\n", i+1, len(all))
		}
		out.WriteString(draft)
	}
	return out.String(), shortened
}

// renderSitting writes one sitting's post, first comment and tagging list,
// cutting the longest subjects until the post fits in limit characters.
//
// They are three blocks because they are pasted into three places. The post
// carries no links, since LinkedIn is widely reported to show a post with an
// outside link to fewer people; the link goes in the first comment. It is the
// channel's parlinfo.ch page, which keeps readers with the project's own
// accounts, where every vote has its card and its result by party; only a
// channel without one gets each entry's official record, numbered like the
// post so a reader can match them. The tagging list is never pasted
// at all: a LinkedIn mention is picked from a dropdown while typing, and the
// profile URL is there to check that the person picked is the one who sits in
// the chamber.
func renderSitting(s sitting, mapper *contacts.Mapper, o options, limit int) (string, int) {
	post, comment, tags, _ := sittingBlocks(s, mapper, o, 0)
	cut := 0
	if chars(post) > limit {
		// The longest subject length at which the post fits.
		lo, hi := shortestSubject, longestSubject(s)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if p, _, _, _ := sittingBlocks(s, mapper, o, mid); chars(p) <= limit {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		post, comment, tags, cut = sittingBlocks(s, mapper, o, lo)
		if n := chars(post); n > limit {
			log.Printf("%s: the post is %d characters even with every title cut to %d, over LinkedIn's %d; shorten it by hand",
				s.heading(), n, shortestSubject, limit)
		}
	}
	return "━━━ Post ━━━\n" + post +
		"\n━━━ First comment ━━━\n" + comment +
		"\n━━━ Tag by hand ━━━\n" + tags, cut
}

// entry is one numbered item of a post: a whole business, or the one vote of
// it a highlight is about.
type entry struct {
	group []votes.Vote
	vote  int // index into group, or -1 for the whole business
}

// part is a run of entries under an optional heading.
type part struct {
	heading string
	entries []entry
}

// parts chooses what a sitting's post lists.
//
// A post listing a long sitting entry by entry reads as a data feed, and the
// Gemeinderat's sitting of 28 October still needed ten titles cut to fit. So a
// sitting of more than maxListed businesses gets the votes worth reading, see
// extremes, and the rest is one click away. A sitting whose source states no
// verdicts gets its closest votes instead.
func parts(s sitting, all bool) []part {
	if all || len(s.Groups) <= maxListed {
		return []part{{entries: whole(s.Groups)}}
	}
	if ps := extremes(s.Groups); len(ps) > 0 {
		return ps
	}
	if closest := highlights(s.Groups, maxListed); len(closest) > 0 {
		return []part{{"Am knappsten:", closest}}
	}
	return []part{{entries: whole(s.Groups)}}
}

// extremes picks a vote for each of four headings: the closest adoption and
// rejection, then the clearest of each. Together they say where the chamber
// was split and where it was of one mind, which five closest votes alone do
// not: on a day the Nationalrat rejects most of what it votes on, those are
// five rejections.
//
// Each business appears once, picked in that order. A sitting with a single
// rejection, or a single adoption, shows it once under a heading saying so,
// since it is both the closest and the clearest. A business stands in by
// whichever of its votes fits the heading. Only a Ja/Nein vote with a stated
// verdict qualifies; see highlights.
func extremes(groups [][]votes.Vote) []part {
	type candidate struct {
		e       entry
		yes, no int
	}
	used := map[*votes.Vote]bool{}
	pick := func(want func(votes.Vote) bool, better func(a, b candidate) bool) []entry {
		var best *candidate
		for _, g := range groups {
			if votes.IsWahlgeschaeftGroup(g) || used[&g[0]] {
				continue
			}
			for i, v := range g {
				yes, no, ok := yesNo(v)
				if !ok || !want(v) {
					continue
				}
				c := candidate{entry{g, i}, yes, no}
				if best == nil || better(c, *best) {
					best = &c
				}
			}
		}
		if best == nil {
			return nil
		}
		used[&best.e.group[0]] = true
		return []entry{best.e}
	}
	// businesses counts the businesses with a vote that qualifies.
	businesses := func(want func(votes.Vote) bool) int {
		n := 0
		for _, g := range groups {
			if votes.IsWahlgeschaeftGroup(g) {
				continue
			}
			for _, v := range g {
				if _, _, ok := yesNo(v); ok && want(v) {
					n++
					break
				}
			}
		}
		return n
	}
	closest := func(a, b candidate) bool { return closer(a.yes, a.no, b.yes, b.no) }
	clearest := func(a, b candidate) bool { return closer(b.yes, b.no, a.yes, a.no) }

	var out []part
	for _, h := range []struct {
		heading, only string
		want          func(votes.Vote) bool
		better        func(a, b candidate) bool
	}{
		{"Am knappsten angenommen:", "Einzige Annahme:", isAdopted, closest},
		{"Am knappsten abgelehnt:", "Einzige Ablehnung:", isRejected, closest},
		{"Am klarsten angenommen:", "", isAdopted, clearest},
		{"Am klarsten abgelehnt:", "", isRejected, clearest},
	} {
		if h.only != "" && businesses(h.want) == 1 {
			h.heading = h.only
		}
		if picked := pick(h.want, h.better); picked != nil {
			out = append(out, part{h.heading, picked})
		}
	}
	return out
}

// whole lists every business as it stands.
func whole(groups [][]votes.Vote) []entry {
	out := make([]entry, len(groups))
	for i, g := range groups {
		out[i] = entry{group: g, vote: -1}
	}
	return out
}

// highlights picks the max closest votes, one per business, closest first,
// whatever their result: the post of a sitting whose source states no
// verdicts, such as the Kantonsrat's.
//
// A business stands in by its closest vote, so six close Änderungsanträge on
// one ordinance do not crowd out everything else. Only a Ja/Nein vote can be
// close: a threshold or Auswahl vote's counts do not measure a margin. An
// election records no result, so it is never one.
func highlights(groups [][]votes.Vote, max int) []entry {
	type candidate struct {
		e       entry
		yes, no int
	}
	var all []candidate
	for _, g := range groups {
		if votes.IsWahlgeschaeftGroup(g) {
			continue
		}
		var best *candidate
		for i, v := range g {
			yes, no, ok := yesNo(v)
			if !ok {
				continue
			}
			if c := (candidate{entry{g, i}, yes, no}); best == nil || closer(c.yes, c.no, best.yes, best.no) {
				best = &c
			}
		}
		if best != nil {
			all = append(all, *best)
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return closer(all[i].yes, all[i].no, all[j].yes, all[j].no) })

	var out []entry
	for _, c := range all {
		if len(out) == max {
			break
		}
		out = append(out, c.e)
	}
	return out
}

// yesNo is a Ja/Nein vote's two counts, and false for any other kind.
func yesNo(v votes.Vote) (yes, no int, ok bool) {
	c := voteformat.CountsOf(v)
	if voteformat.IsAuswahlVote(c) || voteformat.IsQuorumVote(c) || c.Ja == nil || c.Nein == nil || *c.Ja+*c.Nein == 0 {
		return 0, 0, false
	}
	return *c.Ja, *c.Nein, true
}

// closer reports whether a's margin is narrower than b's, as a share of the
// votes cast: 60 to 62 is closer than 600 to 610 would be in a larger chamber.
func closer(aYes, aNo, bYes, bNo int) bool {
	return abs(aYes-aNo)*(bYes+bNo) < abs(bYes-bNo)*(aYes+aNo)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// isRejected reports whether the post states the vote failed.
func isRejected(v votes.Vote) bool {
	return voteformat.HasVerdict(v) && voteformat.GetVoteResultEmoji(voteformat.Verdict(v)) == "❌"
}

// isAdopted reports whether a vote's stated verdict is an adoption.
func isAdopted(v votes.Vote) bool {
	return voteformat.HasVerdict(v) && voteformat.GetVoteResultEmoji(voteformat.Verdict(v)) == "✅"
}

// statsLine counts a sitting: "15 Geschäfte, 16 Abstimmungen: 14 angenommen,
// 2 abgelehnt."
//
// Votes the source reports no verdict for are counted as such rather than
// guessed at, which is most of the Kantonsrat's: the outcome is never inferred
// from the counts, see voteformat.HasVerdict. An election's roll call is not a
// vote on anything, so it counts as a business and nothing else.
func statsLine(groups [][]votes.Vote) string {
	n, accepted, rejected, open := 0, 0, 0, 0
	for _, g := range groups {
		if votes.IsWahlgeschaeftGroup(g) {
			continue
		}
		for _, v := range g {
			n++
			switch {
			case isRejected(v):
				rejected++
			case voteformat.HasVerdict(v):
				accepted++
			default:
				open++
			}
		}
	}
	line := plural(len(groups), "Geschäft", "Geschäfte")
	if n != len(groups) {
		line += ", " + plural(n, "Abstimmung", "Abstimmungen")
	}
	if accepted+rejected > 0 {
		line += fmt.Sprintf(": %d angenommen, %d abgelehnt", accepted, rejected)
		if open > 0 {
			line += fmt.Sprintf(", %d ohne ausgewiesenes Resultat", open)
		}
	}
	return line + "."
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// sittingBlocks renders the three blocks, with every subject cut to at most
// maxSubject characters (0 for no limit), and says how many it cut.
func sittingBlocks(s sitting, mapper *contacts.Mapper, o options, maxSubject int) (post, comment, tags string, cut int) {
	var p, c, t strings.Builder
	fmt.Fprintf(&p, "🗳️ %s\n\n%s\n", s.heading(), statsLine(s.Groups))
	var attributions []string
	seen := map[string]bool{}
	n := 0
	ps := parts(s, o.All)
	for _, pt := range ps {
		p.WriteString("\n")
		if pt.heading != "" {
			p.WriteString(pt.heading + "\n")
		}
		for _, e := range pt.entries {
			n++
			g := e.group
			subject := groupSubject(g)
			if short := shorten(subject, maxSubject); short != subject {
				subject = short
				cut++
			}
			if e.vote < 0 {
				p.WriteString(groupEntry(n, subject, g))
			} else {
				p.WriteString(highlightEntry(n, subject, g, e.vote))
			}

			if link := recordLink(g); link != "" && o.Site == "" {
				fmt.Fprintf(&c, "%d. %s\n", n, link)
			}
			if a := g[0].Attribution; a != "" && !seen[a] {
				seen[a] = true
				attributions = append(attributions, a)
			}
			// An election's title names the member being replaced, not the
			// one elected, so nobody in it is tagged; the other platforms keep
			// the contact mapping away from these posts for the same reason.
			// The people are read from the subject as posted, so a name cut
			// from it is not listed.
			if mapper != nil && !votes.IsWahlgeschaeftGroup(g) {
				for _, m := range mapper.Mentions(subject, "linkedin") {
					fmt.Fprintf(&t, "%d. %s: %s\n", n, m.Name, strings.Join(m.URLs, ", "))
				}
			}
		}
	}
	if o.Site != "" {
		every := "Jede Abstimmung"
		if ps[0].heading != "" {
			every = "Alle " + plural(len(s.Groups), "Geschäft", "Geschäfte")
		}
		fmt.Fprintf(&p, "\n%s einzeln, auf dem Social-Media-Kanal nach Wahl: Link im ersten Kommentar.\n", every)
		fmt.Fprintf(&c, "Alle Abstimmungen einzeln: %s\n", o.Site)
	}
	// The credit travels with the links: two sources' terms require it, and
	// the comment is where this post names where its data came from.
	if len(attributions) > 0 {
		if o.Site == "" {
			c.WriteString("\n")
		}
		c.WriteString(strings.Join(attributions, "\n") + "\n")
	}
	if t.Len() == 0 {
		t.WriteString("Nobody named in these votes has a verified LinkedIn profile on file.\n")
	}
	return p.String(), c.String(), t.String(), cut
}

// highlightEntry is one highlighted vote: its result, the business, and which
// of the business's votes it was where there were several.
func highlightEntry(n int, subject string, group []votes.Vote, i int) string {
	v := group[i]
	lead := tally(v)
	if voteformat.HasVerdict(v) {
		lead = voteformat.GetVoteResultEmoji(voteformat.Verdict(v)) + " " + lead
	}
	which := ""
	if len(group) > 1 {
		// The vote's own label names its type where that matters.
		which = " (" + voteformat.SubVoteLabel(v, i, len(group)) + ")"
	} else if label := voteformat.TypeLabel(v.Type); label != "" {
		lead = label + " · " + lead
	}
	if q := strings.TrimSpace(v.Question); q != "" {
		lead += " · " + q
	}
	return fmt.Sprintf("%d. %s · %s%s\n", n, lead, subject, which)
}

// chars counts a post the way LinkedIn's limit does. It is measured in UTF-16
// units, which is at least the count of characters, so a post this passes is
// never over the limit.
func chars(s string) int {
	return len(utf16.Encode([]rune(strings.TrimRight(s, "\n"))))
}

// longestSubject is the length of the sitting's longest subject.
func longestSubject(s sitting) int {
	longest := 0
	for _, g := range s.Groups {
		if n := len([]rune(groupSubject(g))); n > longest {
			longest = n
		}
	}
	return longest
}

// shorten cuts s to at most max characters at a word boundary, ending in "…".
// A max of 0 leaves s as it is.
func shorten(s string, max int) string {
	r := []rune(s)
	if max <= 0 || len(r) <= max {
		return s
	}
	cut := string(r[:max-1])
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:·–-") + "…"
}

// bySitting orders groups by sitting date, keeping the source's order within
// a day, so each sitting's entries come together under its heading.
func bySitting(groups [][]votes.Vote) [][]votes.Vote {
	out := append([][]votes.Vote(nil), groups...)
	sort.SliceStable(out, func(i, j int) bool { return out[i][0].DateString() < out[j][0].DateString() })
	return out
}

// filingDate is the " vom 07.01.2026" Stadt Zürich's titles give after the
// business and its authors.
var filingDate = regexp.MustCompile(` vom \d{1,2}\.\d{1,2}\.\d{4}`)

// groupSubject is what a group's votes were about: the kind of business and
// who filed it, where the source says, then the title. It is also the text the
// tagging list is read from, so the list names the people the entry names.
func groupSubject(group []votes.Vote) string {
	// The date a business was filed means nothing in a digest of when it was
	// voted on, and on a Gemeinderat title it is a tenth of the length.
	subject := filingDate.ReplaceAllString(voteformat.CleanVoteTitle(group[0].Title), "")
	if prefix := voteformat.GroupPrefixLine(group); prefix != "" {
		subject = prefix + ": " + subject
	}
	return subject
}

// groupEntry is a group's numbered entry in the post. A lone vote is one line,
// result first; several votes on one business share its subject line and take
// a line each below it.
func groupEntry(n int, subject string, group []votes.Vote) string {
	if votes.IsWahlgeschaeftGroup(group) {
		// The roll call on an election records who was present, not who won.
		return fmt.Sprintf("%d. Wahlgeschäft: %s\n", n, subject)
	}
	if len(group) == 1 {
		lead := result(group[0])
		// A threshold vote's tally reads as near-unanimous agreement unless it
		// is named; in a larger group the vote's own label carries the name.
		if label := voteformat.TypeLabel(group[0].Type); label != "" {
			lead = label + " · " + lead
		}
		// What a Ja supported, where the verdict alone would contradict the
		// counts: a business rejected on a large Ja majority, because Ja was
		// the proposal to reject it.
		if q := voteformat.QuestionLine(group); q != "" {
			lead += " · " + q
		}
		return fmt.Sprintf("%d. %s · %s\n", n, lead, subject)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d. %s\n", n, subject)
	for i, v := range group {
		fmt.Fprintf(&sb, "   %s · %s\n", result(v), voteformat.SubVoteLabel(v, i, len(group)))
	}
	return sb.String()
}

// result is a vote's outcome and tally: "✅ Angenommen (72 Ja, 45 Nein)".
//
// A vote with no verdict to state, an Auswahl or a threshold vote, gets its
// counts alone, as on every other platform; see voteformat.HasVerdict.
func result(v votes.Vote) string {
	if !voteformat.HasVerdict(v) {
		return tally(v)
	}
	verdict := voteformat.Verdict(v)
	return fmt.Sprintf("%s %s (%s)",
		voteformat.GetVoteResultEmoji(verdict), voteformat.GetVoteResultText(v.Language, verdict), tally(v))
}

// tally is a vote's counts: "72 Ja, 45 Nein", or the threshold or Auswahl
// counts the other platforms print for those.
func tally(v votes.Vote) string {
	c := voteformat.CountsOf(v)
	if voteformat.IsAuswahlVote(c) || voteformat.IsQuorumVote(c) {
		return strings.TrimPrefix(voteformat.FormatVoteCounts(c), "📊 ")
	}
	w := voteformat.W(v.Language)
	return fmt.Sprintf("%s %s, %s %s",
		voteformat.FormatVoteCount(c.Ja), w.Yes, voteformat.FormatVoteCount(c.Nein), w.No)
}

// recordLink is the one link a group gets in a first comment without a
// parlinfo.ch page: the official
// record of the vote where the parliament publishes one, otherwise the page of
// the business. An election gets the Bulletin.
func recordLink(group []votes.Vote) string {
	// The roll call on an election records attendance, so the only place the
	// outcome can be read is the sitting's Bulletin.
	if votes.IsWahlgeschaeftGroup(group) && group[0].BulletinURL != "" {
		return group[0].BulletinURL
	}
	if len(group) == 1 && group[0].ArchiveURL != "" {
		return group[0].ArchiveURL
	}
	if link := voteformat.GroupLink(group); link != "" {
		return link
	}
	if links := voteformat.GroupLinks(group); len(links) > 0 {
		return links[0].URL
	}
	return ""
}
