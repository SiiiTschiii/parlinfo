package votes

import (
	"errors"
	"fmt"
	"strings"
)

// Process is where a business stands in its parliamentary process, as of the
// vote a post reports: the steps behind it, the step this vote took, and what
// comes next.
//
// It exists because a vote result alone is routinely misread. "✅ Angenommen"
// on a Nationalrat motion reads as decided, when the Ständerat has yet to vote
// on it; a Gemeinderat Postulat that is überwiesen still waits two years for
// the Stadtrat's report. Posts state the next step in one line and show the
// path so far, and that one next step, on a card of its own.
//
// Only the one next step: the path beyond it is long, branches, and is named in
// terms ("Abschreibung", "In Ständerat geplant") that need explaining more than
// they explain.
//
// An adapter fills it from its source's process history plus the outcome of
// the vote itself. The history alone is not enough: every source records the
// step a vote took some time after the vote — parlament.ch within hours, the
// Gemeinderat a week later, the Kantonsrat later still — so at posting time the
// step this vote took is the adapter's choice, made from the result it is
// posting. Everything before it comes from the source.
//
// Every text in a Process is the source's own, verbatim: the status names of
// parlament.ch ("Stellungnahme zum Vorstoss liegt vor", "In Kommission des
// Ständerats"), the Gemeinderat's step names ("Stadtrat, Entgegennahme",
// "Überweisung, Frist 24 Monate"), the chamber's decision ("Annahme"). A
// paraphrase reads differently from what the parliament says and cannot be
// checked against it; "Beantragt Ablehnung" for a government's proposal was
// read as the result of the vote. The step this vote took and the one after it
// are not yet in the source at posting time, so the adapter picks them from the
// same official vocabulary rather than wording them itself: the status or step
// the source records next for a business of this kind, with the details it
// gives — the committee that will deliberate, the deadline the step sets.
//
// The step this vote took is labelled with the chamber's name, its official
// one ("Nationalrat", "Gemeinderat"), and carries the decision as its detail
// ("Annahme", "Überweisung, Frist 24 Monate"), whichever source it comes from:
// a reader looking for the vote finds it the same way on every card.
//
// The one composed text is a next step built from parts the source states:
// "Stadtrat, Weisung, Frist 24 Monate", the step the Gemeinderat records next
// with the deadline its "Überweisung, Frist 24 Monate" set.
//
// The outcome that decides this vote's step must come from a source that
// states it: parlament.ch's reading of what Ja meant (Vote.Outcome), the
// Gemeinderat's Schlussresultat on a business's Schlussabstimmung. Never from
// the counts alone. Ja is whatever proposal was put first, which on the
// Kantonsrat can be a committee's proposal to reject, and OpenParlData does not
// say which; until a source does, the Kantonsrat gets no process.
//
// Nil means the adapter could not say where the business stands, and the post
// goes out as it always has. That is the right answer for anything the adapter
// does not recognise: a wrong next step is worse than none.
type Process struct {
	// Steps run in order: the steps already taken, then exactly one step this
	// vote took, then — unless it ended the business — exactly one step to
	// come. See Validate.
	Steps []ProcessStep

	// Next is what happens next, in the source's words, as a post prints it
	// after "Nächster Schritt:": "In Kommission des Ständerats". When this vote
	// ended the business it is instead the source's word for a closed one —
	// "Erledigt" on parlament.ch, "Abgeschlossen" in the Gemeinderat — under
	// the same label. Required either way; at most MaxNextRunes long.
	Next string
}

// ProcessStep is one stage of a business: who acts, and what they did or will
// do.
type ProcessStep struct {
	// Label names the stage in the source's words: a status
	// ("Eingereicht"), a step ("Stadtrat, Entgegennahme") or the chamber that
	// decided ("Nationalrat"). A card prints it in bold.
	Label string
	// Detail is what the source records about the stage, verbatim: a date, a
	// decision ("Annahme"), a government's proposal ("Antrag des Bundesrates:
	// Ablehnung"). Optional.
	Detail string
	State  StepState
}

// StepState is where a step stands relative to the vote a post reports.
type StepState int

const (
	// StepDone is a step taken before this vote.
	StepDone StepState = iota
	// StepCurrent is the step this vote took, with the business carrying on.
	StepCurrent
	// StepEnded is the step this vote took when it ended the business: a
	// rejected motion, an Einzelinitiative that missed its quorum. No step
	// follows it.
	StepEnded
	// StepNext is the step that comes next. A process has one, after this
	// vote's, unless this vote ended the business.
	StepNext
)

// MaxProcessSteps is the most steps a Process may have. A card shows every
// step, and a path longer than this would leave the business title no room.
// Every process this project models fits well within it; a business whose path
// does not is one to post without a process rather than squeeze.
const MaxProcessSteps = 6

// MaxNextRunes is the longest next-step text a Process may carry.
//
// The sentence goes out whole, as one block of an X or Bluesky reply, and the
// reply packer never splits a block: one longer than a post would make the
// first reply fail after the root had already been published. 140 runes leaves
// that reply room under X's 280 weighted characters for the line's emoji and
// label, and keeps the card's footer to its two lines. Next steps are meant to
// be one short sentence; the longest any body needs today is under 100.
const MaxNextRunes = 140

// Ended reports whether the vote ended the business.
func (p Process) Ended() bool {
	for _, s := range p.Steps {
		if s.State == StepEnded {
			return true
		}
	}
	return false
}

// Validate checks that p tells one coherent story: done steps, then the one
// step this vote took, then — unless that step ended the business — the one
// step that comes next, and a next-step text in either case.
//
// Adapters build processes from rules and source data that can disagree with
// each other; a process that fails here is dropped rather than posted.
func (p Process) Validate() error {
	if len(p.Steps) == 0 {
		return errors.New("process has no steps")
	}
	if len(p.Steps) > MaxProcessSteps {
		return fmt.Errorf("process has %d steps, more than %d", len(p.Steps), MaxProcessSteps)
	}

	current, next := -1, 0
	for i, s := range p.Steps {
		if strings.TrimSpace(s.Label) == "" {
			return fmt.Errorf("step %d has no label", i+1)
		}
		switch s.State {
		case StepDone:
			if current >= 0 {
				return fmt.Errorf("step %d is done but follows this vote's step", i+1)
			}
		case StepCurrent, StepEnded:
			if current >= 0 {
				return fmt.Errorf("step %d is a second step for this vote", i+1)
			}
			current = i
		case StepNext:
			if current < 0 {
				return fmt.Errorf("step %d is still to come but precedes this vote's step", i+1)
			}
			next++
		default:
			return fmt.Errorf("step %d has unknown state %d", i+1, s.State)
		}
	}
	if current < 0 {
		return errors.New("process has no step for this vote")
	}

	switch {
	case p.Steps[current].State == StepEnded && next > 0:
		return errors.New("steps follow the step that ended the business")
	case p.Steps[current].State == StepCurrent && next != 1:
		return fmt.Errorf("an open business shows exactly one next step, not %d", next)
	}
	text := strings.TrimSpace(p.Next)
	if text == "" {
		return errors.New("a process needs a next step, or the source's word for a closed business")
	}
	if n := len([]rune(text)); n > MaxNextRunes {
		return fmt.Errorf("next step is %d runes, more than %d", n, MaxNextRunes)
	}
	return nil
}

// Equal reports whether p and q describe the same process.
func (p *Process) Equal(q *Process) bool {
	if p == nil || q == nil {
		return p == q
	}
	if p.Next != q.Next || len(p.Steps) != len(q.Steps) {
		return false
	}
	for i := range p.Steps {
		if p.Steps[i] != q.Steps[i] {
			return false
		}
	}
	return true
}

// GroupProcess returns the process a posting group reports, or nil when there
// is none to report.
//
// A group is one business on one day, so its votes should all carry the same
// process; the adapter sets it from the group's decisive vote. When they do not
// — one vote without a process, or two that disagree about where the business
// stands — nothing is reported rather than a guess at which one is right. An
// invalid process is likewise reported as none, and so is any process on an
// election business, which records no decision a process could follow from.
func GroupProcess(group []Vote) *Process {
	if len(group) == 0 || IsWahlgeschaeftGroup(group) {
		return nil
	}
	p := group[0].Affair.Process
	if p == nil || p.Validate() != nil {
		return nil
	}
	for _, v := range group[1:] {
		if !p.Equal(v.Affair.Process) {
			return nil
		}
	}
	return p
}
