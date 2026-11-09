package voteformat

import (
	"fmt"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// Tags rewrites text to tag the accounts it names. Each func takes the text and
// the jurisdiction it belongs to, since a Fraktion name maps to a different party
// account in each chamber. A nil func leaves its text untouched.
type Tags struct {
	// Heading tags the title and the label line, the lines that name members.
	Heading func(text, jurisdiction string) string
	// SubVote tags each vote's label in a multi-vote group. Those name
	// Fraktionen at most — "Antrag der SVP-Fraktion" — never members.
	SubVote func(text, jurisdiction string) string
}

func orIdentity(f func(string, string) string) func(string, string) string {
	if f == nil {
		return func(s, _ string) string { return s }
	}
	return f
}

// LongFormBody renders a group as one block of text, without its links: the
// heading, the co-signers, then per vote the verdict, the counts and the
// Fraktion breakdown.
//
// The business's next step (NextStepLine) is not part of it: callers put it
// between the body and the links, where a caption cut for length keeps it.
//
// It is what the platforms that publish a vote as a single post carry — the
// Instagram caption and the Facebook message — where X and Bluesky spread the
// same facts over a thread. Sharing it keeps the two from drifting apart line
// by line.
//
// tags mark up the names in the body for platforms that can tag accounts; the
// zero value leaves the text as it is.
//
// A Wahlgeschäft is not rendered here: its only votes are attendance roll
// calls, and nothing below means anything for a headcount. Callers check
// votes.IsWahlgeschaeftGroup first.
func LongFormBody(group []votes.Vote, tags Tags) string {
	var sb strings.Builder
	sb.WriteString(LongFormHeading(group, tags))
	sb.WriteString(LongFormCoSigners(group, tags))
	for i := range group {
		sb.WriteString(LongFormVote(group, i, tags))
		if i < len(group)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// LongFormHeading is the top of LongFormBody: the headline, the label line —
// what kind of business this is and who filed it, plus the
// Abstimmungsgegenstand when a lone vote makes that meaningful — and the title,
// ending in the blank line before the first vote.
//
// The label line is tagged like the title, and for a body whose titles carry no
// names — Kanton Zürich's never do — it is the only line that names anyone at
// all.
func LongFormHeading(group []votes.Vote, tags Tags) string {
	tagHeading := orIdentity(tags.Heading)
	jurisdiction := group[0].Jurisdiction

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🗳️ %s\n\n", PostHeadline(group)))
	if prefix := GroupPrefixLine(group); prefix != "" {
		sb.WriteString(tagHeading(prefix, jurisdiction))
		sb.WriteString("\n")
	}
	sb.WriteString(tagHeading(CleanVoteTitle(group[0].Title), jurisdiction))
	if q := QuestionLine(group); q != "" {
		sb.WriteString("\n" + q)
	}
	sb.WriteString("\n\n")
	return sb.String()
}

// LongFormCoSigners is the co-signers below the heading, tagged like it and
// ending in a blank line; empty when there are none.
//
// Not part of the heading, which is also the card's alt text: the card names
// who filed and who took over, never the co-signers.
func LongFormCoSigners(group []votes.Vote, tags Tags) string {
	line := CoSignerLine(group)
	if line == "" {
		return ""
	}
	return orIdentity(tags.Heading)(line, group[0].Jurisdiction) + "\n\n"
}

// LongFormVote is the part of LongFormBody that belongs to vote i of the group:
// its label and verdict, its counts and its Fraktion breakdown, ending in a
// newline. A platform that shows each vote beside its own card uses it on its
// own.
func LongFormVote(group []votes.Vote, i int, tags Tags) string {
	tagSubVote := orIdentity(tags.SubVote)
	vote := group[i]
	counts := CountsOf(vote)

	var sb strings.Builder
	if len(group) > 1 {
		// Multi-vote: include subtitle
		voteTitle := tagSubVote(SubVoteLabel(vote, i, len(group)), vote.Jurisdiction)
		if !HasVerdict(vote) {
			sb.WriteString(voteTitle)
		} else {
			sb.WriteString(fmt.Sprintf("%s %s: %s",
				GetVoteResultEmoji(Verdict(vote)), GetVoteResultText(Verdict(vote)), voteTitle))
		}
		sb.WriteString("\n")
	} else {
		// Single vote: the ballot type when it is one worth naming, then the
		// result line. A lone threshold vote needs the label most — there is
		// no sibling beside it to make the lopsided tally look unusual.
		if label := TypeLabel(vote.Type); label != "" {
			sb.WriteString(label + "\n")
		}
		if HasVerdict(vote) {
			sb.WriteString(fmt.Sprintf("%s %s\n",
				GetVoteResultEmoji(Verdict(vote)), GetVoteResultText(Verdict(vote))))
		}
	}

	sb.WriteString(FormatVoteCountsLong(counts))
	sb.WriteString("\n")

	if len(vote.MemberVotes) > 0 {
		if breakdown := FormatFraktionBreakdown(AggregateFraktionCounts(vote)); breakdown != "" {
			sb.WriteString("\n")
			sb.WriteString(breakdown)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
