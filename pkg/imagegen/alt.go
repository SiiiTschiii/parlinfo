package imagegen

import (
	"fmt"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The alt text of each card: what the card says, as text, for a reader who
// cannot see it. Each is built from the voteformat pieces the Instagram caption
// uses, so the description and the caption state the same facts.
//
// Every description is a label part and a result part, and the label gives way
// first. A business title or vote label can run to well over a thousand
// characters, and a description capped at the end would then lose the verdict
// and the counts — the very things the card is there to show. The label keeps
// a floor of its own all the same, since an overview of ten votes can list more
// results than the cap holds, and a description that names no business is no
// description of this card.

// altLabelFloor is how much of the label survives however long the result.
const altLabelFloor = 300

// assembleAlt joins label and result within maxAltRunes, shortening the label
// down to altLabelFloor before the result loses anything.
func assembleAlt(label, result string) string {
	label = strings.TrimSpace(label)
	result = strings.TrimSpace(result)
	if label == "" || result == "" {
		return capRunes(label+result, maxAltRunes)
	}
	labelRoom := maxAltRunes - len([]rune(result)) - 2 // "\n\n"
	if labelRoom < altLabelFloor {
		labelRoom = altLabelFloor
	}
	label = capRunes(label, labelRoom)
	return label + "\n\n" + capRunes(result, maxAltRunes-len([]rune(label))-2)
}

// wahlgeschaeftAlt describes the election notice card.
func wahlgeschaeftAlt(group []votes.Vote) string {
	return assembleAlt(fmt.Sprintf("🗳️ %s", voteformat.PostHeadline(group)), voteformat.WahlgeschaeftBody(group))
}

// combinedAlt describes a lone vote's card: the heading, then the outcome and
// the counts.
func combinedAlt(group []votes.Vote) string {
	result := voteformat.LongFormVote(group, 0, voteformat.Tags{})
	if outcome := auswahlOutcome(group[0]); outcome != "" {
		result = outcome + "\n" + result
	}
	return assembleAlt(voteformat.LongFormHeading(group, voteformat.Tags{}), result)
}

// titleAlt describes the overview card: the heading, then the same one-line
// result per vote that the card lists. The results are what a reader of this
// card learns that the heading does not say, and the cards with the details may
// sit several replies further down the thread.
//
// Every vote keeps its line. Where the lines at the card's label length would
// not fit beside the heading, the labels are cut shorter, down to the last step,
// rather than the list losing its tail.
func titleAlt(group []votes.Vote) string {
	var result string
	for _, maxRunes := range []int{summarySubtitleMaxRunes, 60, 40, 20} {
		var lines []string
		for i, v := range group {
			if line, ok := formatSummaryLineWithin(i+1, v, len(group), maxRunes); ok {
				lines = append(lines, line)
			}
		}
		result = strings.Join(lines, "\n")
		if len([]rune(result)) <= maxAltRunes-altLabelFloor-2 {
			break
		}
	}
	return assembleAlt(voteformat.LongFormHeading(group, voteformat.Tags{}), result)
}

// resultAlt describes vote i's result card: its label and verdict, then its
// counts and Fraktion breakdown.
func resultAlt(group []votes.Vote, i int) string {
	v := group[i]
	label := voteformat.SubVoteLabel(v, i, len(group))
	switch {
	case auswahlOutcome(v) != "":
		label = auswahlOutcome(v) + ": " + label
	case voteformat.HasVerdict(v):
		label = fmt.Sprintf("%s %s: %s", voteformat.GetVoteResultEmoji(v.Decision), voteformat.GetVoteResultText(v.Decision), label)
	}

	result := voteformat.FormatVoteCountsLong(voteformat.CountsOf(v))
	if len(v.MemberVotes) > 0 {
		if breakdown := voteformat.FormatFraktionBreakdown(voteformat.AggregateFraktionCounts(v)); breakdown != "" {
			result += "\n\n" + breakdown
		}
	}
	return assembleAlt(label, result)
}

// auswahlOutcome is the option an Auswahl vote chose, as the source reports it
// ("Auswahl A"), or "" for any other vote. The card prints it where a Ja/Nein
// vote prints its verdict, and LongFormVote, having no verdict to state for it,
// leaves it out.
func auswahlOutcome(v votes.Vote) string {
	if !voteformat.IsAuswahlVote(voteformat.CountsOf(v)) {
		return ""
	}
	return strings.TrimSpace(v.Decision)
}
