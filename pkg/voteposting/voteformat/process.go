package voteformat

import (
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// NextStepLabel opens the line saying what happens to a business next. It is
// the one piece of the line that is ours; what follows it is the source's.
const NextStepLabel = "Nächster Schritt:"

// NextStep is what a group's posts say about the business's next step, as the
// label and the source's text: ("Nächster Schritt:", "In Kommission des
// Ständerats") for a business that goes on. For one this vote ended there is
// no next step, so the label is the source's own heading for a business's
// status instead — ("Stand der Beratungen:", "Erledigt") as parlament.ch's
// business page prints it. A bare "Erledigt" would not say what is settled.
// Both are empty when the group reports no process; see votes.GroupProcess.
//
// The card and the text posts render it each in their own way — the card sets
// the label in bold — so it is returned in parts rather than as one line.
func NextStep(group []votes.Vote) (label, text string) {
	p := votes.GroupProcess(group)
	if p == nil {
		return "", ""
	}
	if p.Ended() {
		return strings.TrimSpace(p.StatusLabel), strings.TrimSpace(p.Next)
	}
	return NextStepLabel, strings.TrimSpace(p.Next)
}

// NextStepLine is NextStep as a text post prints it, or "" when the group
// reports no process: "➡️ Nächster Schritt: In Kommission des Ständerats", or
// for a business this vote ended "🏁 Stand der Beratungen: Erledigt".
func NextStepLine(group []votes.Vote) string {
	label, text := NextStep(group)
	if text == "" {
		return ""
	}
	marker := "➡️"
	if votes.GroupProcess(group).Ended() {
		marker = "🏁"
	}
	return marker + " " + label + " " + text
}
