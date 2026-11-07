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
	return assembleAltWithFloor(label, result, altLabelFloor)
}

// assembleAltWithFloor is assembleAlt with the label shortened down to floor
// rather than altLabelFloor.
func assembleAltWithFloor(label, result string, floor int) string {
	label = strings.TrimSpace(label)
	result = strings.TrimSpace(result)
	if label == "" || result == "" {
		return capRunes(label+result, maxAltRunes)
	}
	labelRoom := maxAltRunes - len([]rune(result)) - 2 // "\n\n"
	if labelRoom < floor {
		labelRoom = floor
	}
	label = capRunes(label, labelRoom)
	return label + "\n\n" + capRunes(result, maxAltRunes-len([]rune(label))-2)
}

// wahlgeschaeftAlt describes the election notice card. The business title is
// label, not result: it can run long, and the sentence saying we have no result
// is what has to survive the cap, as it does on the card.
func wahlgeschaeftAlt(group []votes.Vote) string {
	label := fmt.Sprintf("🗳️ %s\n\n%s", voteformat.PostHeadline(group), voteformat.CleanVoteTitle(group[0].Title))
	return assembleAlt(label, voteformat.WahlgeschaeftNoResult)
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

// titleAlt describes the overview card: the heading, then what the card
// shows below the title — how many votes there were, and one line per vote
// with its result and tally. The results are what a reader of this card
// learns that the heading does not say, and the cards with the details may sit
// several replies further down the thread.
//
// Every vote keeps its line and its tally. Where the lines at the card's label
// length would not fit beside the heading, the labels are cut shorter, then
// left out, rather than the list losing its tail. If the results still do not
// fit, the heading gives up room beyond altLabelFloor, down to
// overviewAltHeadingFloor: a group that long is named by its business number
// and the start of its title, and every result stays.
func titleAlt(group []votes.Vote) string {
	head := fmt.Sprintf("%d Abstimmungen\n", len(group))
	// Labels shorten first, then go, then the tallies take their short form:
	// what a vote is called can be found on its own card, its result is what
	// this description is for.
	steps := []struct {
		maxRunes int // 0: no label
		long     bool
	}{
		{summarySubtitleMaxRunes, true}, {60, true}, {40, true}, {20, true},
		{0, true}, {0, false},
	}
	var result string
	for _, step := range steps {
		var lines []string
		for i, v := range group {
			tally := overviewTally(v)
			if step.long {
				tally = overviewTallyLong(v)
			}
			if step.maxRunes == 0 {
				lines = append(lines, overviewAltPrefix(i+1, v)+" – "+tally)
				continue
			}
			if line, ok := formatSummaryLineWithin(i+1, v, len(group), step.maxRunes); ok {
				lines = append(lines, line+" – "+tally)
			}
		}
		result = head + strings.Join(lines, "\n")
		if len([]rune(result)) <= maxAltRunes-altLabelFloor-2 {
			break
		}
	}
	floor := maxAltRunes - len([]rune(result)) - 2
	floor = max(min(floor, altLabelFloor), overviewAltHeadingFloor)
	return assembleAltWithFloor(voteformat.LongFormHeading(group, voteformat.Tags{}), result, floor)
}

// overviewAltHeadingFloor is the least of the heading an overview description
// keeps however many results it lists.
const overviewAltHeadingFloor = 80

// overviewAltPrefix is a vote's number and verdict without its label, for an
// overview description too long to name each vote.
func overviewAltPrefix(index int, v votes.Vote) string {
	switch {
	case auswahlOutcome(v) != "":
		return fmt.Sprintf("%d. %s", index, auswahlResultLabel(v.Decision))
	case voteformat.HasVerdict(v):
		return fmt.Sprintf("%d. %s", index, voteformat.GetVoteResultEmoji(voteformat.Verdict(v)))
	}
	return fmt.Sprintf("%d.", index)
}

// resultAlt describes vote i's result card: when it was taken, its label and
// verdict, then its counts and Fraktion breakdown. The timestamp is the one the
// card prints in its corner; a result card posted on its own reply has nothing
// else to date it by.
func resultAlt(group []votes.Vote, i int) string {
	v := group[i]
	label := voteformat.SubVoteLabel(v, i, len(group))
	switch {
	case auswahlOutcome(v) != "":
		label = auswahlOutcome(v) + ": " + label
	case voteformat.HasVerdict(v):
		label = fmt.Sprintf("%s %s: %s", voteformat.GetVoteResultEmoji(voteformat.Verdict(v)), voteformat.GetVoteResultText(v.Language, voteformat.Verdict(v)), label)
	}

	if stamp := voteformat.FormatVoteDateTime(v.Date); stamp != "" {
		label = stamp + "\n" + label
	}

	result := voteformat.FormatVoteCountsLong(voteformat.CountsOf(v))
	if len(v.MemberVotes) > 0 {
		if breakdown := voteformat.FormatFraktionBreakdown(voteformat.AggregateFraktionCounts(v), v.Language); breakdown != "" {
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
