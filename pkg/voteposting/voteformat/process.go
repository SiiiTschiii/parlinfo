package voteformat

import (
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// NextStepLabel opens the line saying what happens to a business next.
const NextStepLabel = "Als Nächstes:"

// ProcessEnded is what a post says in place of a next step when the vote ended
// the business. The verdict above it says how the vote went; this says that
// nothing follows, which the verdict alone does not — a motion rejected in the
// Nationalrat reads no differently from one the Ständerat may still revive.
const ProcessEnded = "Das Geschäft ist erledigt."

// NextStep is what a group's posts say about the business's next step, as a
// label and a sentence: ("Als Nächstes:", "Der Ständerat entscheidet.") for a
// business that goes on, ("", ProcessEnded) for one this vote ended. Both are
// empty when the group reports no process; see votes.GroupProcess.
//
// The card and the text posts render it each in their own way — the card sets
// the label in bold — so it is returned in parts rather than as one line.
func NextStep(group []votes.Vote) (label, text string) {
	p := votes.GroupProcess(group)
	if p == nil {
		return "", ""
	}
	if p.Ended() {
		return "", ProcessEnded
	}
	return NextStepLabel, strings.TrimSpace(p.Next)
}

// NextStepLine is NextStep as a text post prints it, or "" when the group
// reports no process: "➡️ Als Nächstes: Der Ständerat entscheidet." or
// "🏁 Das Geschäft ist erledigt."
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
