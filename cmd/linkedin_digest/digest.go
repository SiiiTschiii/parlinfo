package main

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

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

// sitting is one body's votes on one day: what one LinkedIn post covers.
type sitting struct {
	Body   string
	Date   time.Time
	Groups [][]votes.Vote
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

// render writes the draft: one post per sitting, each with its first comment
// and its tagging list. It also returns how many entries it shortened to make
// a post fit.
//
// A post per sitting, because a week of one parliament does not fit in one:
// the Gemeinderat's sitting of 28 October alone came to 3,595 characters
// against LinkedIn's 3,000.
//
// mapper may be nil, which leaves the tagging lists empty.
func render(sections []section, mapper *contacts.Mapper) (string, int) {
	all := sittings(sections)
	var out strings.Builder
	shortened := 0
	for i, s := range all {
		draft, cut := renderSitting(s, mapper, maxPostChars)
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
// outside link to fewer people; the links go in the first comment, numbered
// like the post so a reader can match them. The tagging list is never pasted
// at all: a LinkedIn mention is picked from a dropdown while typing, and the
// profile URL is there to check that the person picked is the one who sits in
// the chamber.
func renderSitting(s sitting, mapper *contacts.Mapper, limit int) (string, int) {
	post, comment, tags, _ := sittingBlocks(s, mapper, 0)
	cut := 0
	if chars(post) > limit {
		// The longest subject length at which the post fits.
		lo, hi := shortestSubject, longestSubject(s)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if p, _, _, _ := sittingBlocks(s, mapper, mid); chars(p) <= limit {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		post, comment, tags, cut = sittingBlocks(s, mapper, lo)
		if n := chars(post); n > limit {
			log.Printf("%s: the post is %d characters even with every title cut to %d, over LinkedIn's %d; shorten it by hand",
				sittingHeading(s.Body, s.Date), n, shortestSubject, limit)
		}
	}
	return "━━━ Post ━━━\n" + post +
		"\n━━━ First comment ━━━\n" + comment +
		"\n━━━ Tag by hand ━━━\n" + tags, cut
}

// sittingBlocks renders the three blocks, with every subject cut to at most
// maxSubject characters (0 for no limit), and says how many it cut.
func sittingBlocks(s sitting, mapper *contacts.Mapper, maxSubject int) (post, comment, tags string, cut int) {
	var p, c, t strings.Builder
	fmt.Fprintf(&p, "🗳️ %s\n\n", sittingHeading(s.Body, s.Date))
	var attributions []string
	seen := map[string]bool{}
	for i, g := range s.Groups {
		n := i + 1
		subject := groupSubject(g)
		if short := shorten(subject, maxSubject); short != subject {
			subject = short
			cut++
		}
		p.WriteString(groupEntry(n, subject, g))

		if link := recordLink(g); link != "" {
			fmt.Fprintf(&c, "%d. %s\n", n, link)
		}
		if a := g[0].Attribution; a != "" && !seen[a] {
			seen[a] = true
			attributions = append(attributions, a)
		}
		// An election's title names the member being replaced, not the one
		// elected, so nobody in it is tagged; the other platforms keep the
		// contact mapping away from these posts for the same reason. The
		// people are read from the subject as posted, so a name cut from it
		// is not listed.
		if mapper != nil && !votes.IsWahlgeschaeftGroup(g) {
			for _, m := range mapper.Mentions(subject, "linkedin") {
				fmt.Fprintf(&t, "%d. %s: %s\n", n, m.Name, strings.Join(m.URLs, ", "))
			}
		}
	}
	// The credit travels with the links: two sources' terms require it, and
	// the comment is where this post names where its data came from.
	if len(attributions) > 0 {
		c.WriteString("\n" + strings.Join(attributions, "\n") + "\n")
	}
	if t.Len() == 0 {
		t.WriteString("Nobody named in these votes has a verified LinkedIn profile on file.\n")
	}
	return p.String(), c.String(), t.String(), cut
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

// sittingHeading names the body and the day it sat: "Kantonsrat Zürich |
// Sitzung vom 09.11.2026", or the body alone where the source gave no date.
func sittingHeading(body string, date time.Time) string {
	if date.IsZero() {
		return body
	}
	return body + " | Sitzung vom " + voteformat.FormatVoteDate(date)
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
	c := voteformat.CountsOf(v)
	tally := strings.TrimPrefix(voteformat.FormatVoteCounts(c), "📊 ")
	if !voteformat.IsAuswahlVote(c) && !voteformat.IsQuorumVote(c) {
		w := voteformat.W(v.Language)
		tally = fmt.Sprintf("%s %s, %s %s",
			voteformat.FormatVoteCount(c.Ja), w.Yes, voteformat.FormatVoteCount(c.Nein), w.No)
	}
	if !voteformat.HasVerdict(v) {
		return tally
	}
	verdict := voteformat.Verdict(v)
	return fmt.Sprintf("%s %s (%s)",
		voteformat.GetVoteResultEmoji(verdict), voteformat.GetVoteResultText(v.Language, verdict), tally)
}

// recordLink is the one link a group gets in the first comment: the official
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
