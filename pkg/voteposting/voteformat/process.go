package voteformat

import (
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// NextStepLabel opens the line saying what happens to a business next.
const NextStepLabel = "Als Nächstes:"

// NextStep is what a group's posts say about the business's next step, as a
// label and the source's text: ("Als Nächstes:", "In Kommission des
// Ständerats") for a business that goes on, ("", "Erledigt") for one this vote
// ended — the source's own word for a closed business, which says what the
// verdict alone does not: that nothing follows. Both are empty when the group
// reports no process; see votes.GroupProcess.
//
// The card and the text posts render it each in their own way — the card sets
// the label in bold — so it is returned in parts rather than as one line.
func NextStep(group []votes.Vote) (label, text string) {
	p := votes.GroupProcess(group)
	if p == nil {
		return "", ""
	}
	if p.Ended() {
		return "", strings.TrimSpace(p.Next)
	}
	return NextStepLabel, strings.TrimSpace(p.Next)
}

// NextStepLine is NextStep as a text post prints it, or "" when the group
// reports no process: "➡️ Als Nächstes: In Kommission des Ständerats" or
// "🏁 Erledigt".
func NextStepLine(group []votes.Vote) string {
	label, text := NextStep(group)
	switch {
	case text == "":
		return ""
	case label == "":
		return "🏁 " + text
	}
	return "➡️ " + label + " " + text
}
