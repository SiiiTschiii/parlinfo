package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// postedSince collects the IDs of the votes any of the logs records as posted
// at or after since.
//
// Any platform counts. The digest is about what the bot published, and a vote
// that reached one account and failed on another was still published.
func postedSince(logs []*votelog.VoteLog, since time.Time) map[string]bool {
	ids := map[string]bool{}
	for _, l := range logs {
		for _, e := range l.Votes {
			if !e.PostedAt.Before(since) {
				ids[e.ID] = true
			}
		}
	}
	return ids
}

// keepPosted narrows groups to the votes in ids, dropping a group left empty.
//
// It runs after grouping because grouping can pull in votes that were never
// fetched: PARIS completes a Geschäft with its earlier agenda items, and one of
// those the bot skipped must not appear in a digest of what it posted.
func keepPosted(groups [][]votes.Vote, ids map[string]bool) [][]votes.Vote {
	var out [][]votes.Vote
	for _, g := range groups {
		var kept []votes.Vote
		for _, v := range g {
			if ids[v.SourceID] {
				kept = append(kept, v)
			}
		}
		if len(kept) > 0 {
			out = append(out, kept)
		}
	}
	return out
}

// missing lists the IDs no group holds, sorted: votes the logs say were posted
// that the source did not return, which a larger fetch usually finds.
func missing(ids map[string]bool, groups [][]votes.Vote) []string {
	found := map[string]bool{}
	for _, g := range groups {
		for _, v := range g {
			found[v.SourceID] = true
		}
	}
	var out []string
	for id := range ids {
		if !found[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// section is one body's part of the digest.
type section struct {
	Name   string
	Groups [][]votes.Vote
}

// render writes the draft: the post, the first comment and the tagging list.
//
// They are three blocks because they are pasted into three places. The post
// carries no links, since LinkedIn is widely reported to show a post with an
// outside link to fewer people; the links go in the first comment, numbered
// like the post so a reader can match them. The tagging list is never pasted
// at all: a LinkedIn mention is picked from a dropdown while typing, and the
// profile URL is there to check that the person picked is the one who sits in
// the chamber.
//
// mapper may be nil, which leaves the tagging list empty.
func render(sections []section, since, until time.Time, mapper *contacts.Mapper) string {
	var post, comment, tags strings.Builder
	attributions := []string{}
	seenAttribution := map[string]bool{}

	n := 0
	for _, s := range sections {
		if len(s.Groups) == 0 {
			continue
		}
		if post.Len() > 0 {
			post.WriteString("\n")
		}
		fmt.Fprintf(&post, "🗳️ %s | %s – %s\n\n", s.Name,
			voteformat.FormatVoteDate(since), voteformat.FormatVoteDate(until))

		for _, g := range s.Groups {
			n++
			subject := groupSubject(g)
			post.WriteString(groupEntry(n, subject, g))

			if link := recordLink(g); link != "" {
				fmt.Fprintf(&comment, "%d. %s\n", n, link)
			}
			if a := g[0].Attribution; a != "" && !seenAttribution[a] {
				seenAttribution[a] = true
				attributions = append(attributions, a)
			}
			// An election's title names the member being replaced, not the one
			// elected, so nobody in it is tagged; the other platforms keep the
			// contact mapping away from these posts for the same reason.
			if mapper != nil && !votes.IsWahlgeschaeftGroup(g) {
				for _, m := range mapper.Mentions(subject, "linkedin") {
					fmt.Fprintf(&tags, "%d. %s: %s\n", n, m.Name, strings.Join(m.URLs, ", "))
				}
			}
		}
	}

	if n == 0 {
		return ""
	}
	// The credit travels with the links: two sources' terms require it, and the
	// comment is where this post names where its data came from.
	if len(attributions) > 0 {
		comment.WriteString("\n" + strings.Join(attributions, "\n") + "\n")
	}
	if tags.Len() == 0 {
		tags.WriteString("Nobody named in these votes has a verified LinkedIn profile on file.\n")
	}

	return "━━━ Post ━━━\n" + post.String() +
		"\n━━━ First comment ━━━\n" + comment.String() +
		"\n━━━ Tag by hand ━━━\n" + tags.String()
}

// groupSubject is what a group's votes were about: the kind of business and
// who filed it, where the source says, then the title. It is also the text the
// tagging list is read from, so the list names the people the entry names.
func groupSubject(group []votes.Vote) string {
	subject := voteformat.CleanVoteTitle(group[0].Title)
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
		return fmt.Sprintf("%d. %s · %s\n", n, result(group[0]), subject)
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
// the business.
func recordLink(group []votes.Vote) string {
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
