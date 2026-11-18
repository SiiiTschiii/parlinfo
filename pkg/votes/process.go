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
// ("Annahme", "Überweisung"), whichever source it comes from:
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
// Where the adapter knows the path up to this vote but not the step after it —
// a Gesamtabstimmung, after which a bill can go to the other chamber, into the
// Differenzbereinigung or to the Schlussabstimmung — the process ends at this
// vote's step and says nothing of what follows (Open). It may instead carry a
// line that is true whichever way the business goes, under a heading of its
// own: "Noch nicht endgültig: Schlussabstimmung beider Räte steht aus" after a
// Gesamtabstimmung on a federal law, which is never adopted without one. That
// line and its heading are the adapter's, not the source's; it is the one
// place a process words a status itself, and only where the law makes it
// certain.
//
// Nil means the adapter could not say where the business stands, and the post
// goes out as it always has. That is the right answer for anything the adapter
// does not recognise: a wrong next step is worse than none.
type Process struct {
	// Steps run in order: the steps already taken, then exactly one step this
	// vote took, then — unless it ended the business or the step after it is
	// not known — exactly one step to come. See Validate.
	Steps []ProcessStep

	// Next is what happens next, in the source's words, as a post prints it
	// after "Nächster Schritt:": "In Kommission des Ständerats". When this vote
	// ended the business it is instead the source's word for a closed one —
	// "Erledigt" on parlament.ch, "Abgeschlossen" in the Gemeinderat — under
	// StatusLabel. Required either way; at most MaxNextRunes long. A process
	// with no step to come (Open) may leave it empty, or carry a line under
	// StatusLabel that holds whatever comes next.
	Next string

	// StatusLabel is the heading a post prints before Next instead of
	// "Nächster Schritt:" where there is no next step to name. When this vote
	// ended the business it is the source's own heading for a business's
	// status: "Stand der Beratungen:" on parlament.ch's business page,
	// "Geschäftsstatus:" for PARIS's Geschaeftsstatus. Nothing comes next for
	// a closed business, so "Nächster Schritt: Erledigt" answered a question
	// it had just made moot. On an Open process with a Next it heads that
	// line: "Noch nicht endgültig:". Required in both cases; unused otherwise.
	StatusLabel string

	// Kind is the business type as the source names it ("Postulat"), for the
	// status card's heading where the vote itself carries none. Optional.
	Kind string

	// Bill names which of a business's bills the vote was on, where it has
	// more than one: "Entwurf 1 von 4". The history is that bill's alone, and
	// the heading says so. Optional.
	Bill string

	// About is the source's own sentence on what a business of this type
	// asks for: "Mit einer Motion wird der Bundesrat beauftragt, …" from
	// parlament.ch, "Ein Postulat fordert den Stadtrat auf zu prüfen, …" from
	// the Gemeinderat. Without it "Überwiesen an den Bundesrat" leaves open
	// what the Bundesrat now has to do. Optional.
	About string

	// VoteAbout is the source's own sentence on the kind of vote this was,
	// where that kind decides what the result means: parlament.ch's
	// Parlamentswörterbuch on the Gesamtabstimmung, "Die Erstberatung eines
	// Erlassentwurfes endet mit der Gesamtabstimmung.", which a reader would
	// otherwise take for the final word. Optional; a card short of room
	// leaves it out first.
	VoteAbout string
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
	// Skipped is how many steps a StepSkipped placeholder stands for.
	Skipped int
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
	// StepCompleted is the step this vote took when it brought the business
	// to its end in its favour: an adopted Schlussabstimmung. Like StepEnded
	// no step follows it, but it is a step taken, not one that failed.
	StepCompleted
	// StepSkipped stands for steps taken before this vote that the card has
	// no room for, so that leaving them out shows. It has no label of its own;
	// Skipped says how many it replaces. See TrimHistory.
	StepSkipped
)

// closes reports whether a state is this vote's step ending the business.
func (s StepState) closes() bool { return s == StepEnded || s == StepCompleted }

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

// TrimHistory shortens the steps taken before a vote to at most room,
// keeping the first — where the business starts — and the most recent, which
// lead up to the vote, with a StepSkipped placeholder between them for the
// ones left out, so a reader sees that the path had more. The placeholder
// takes one of the room's slots. A business with a long history keeps its
// process rather than losing it to MaxProcessSteps; what it gives up is the
// middle of a path the source still lists in full.
func TrimHistory(done []ProcessStep, room int) []ProcessStep {
	if room <= 0 {
		return nil
	}
	if len(done) <= room {
		return done
	}
	if room < 3 {
		return done[len(done)-room:]
	}
	recent := done[len(done)-room+2:]
	gap := ProcessStep{State: StepSkipped, Skipped: len(done) - 1 - len(recent)}
	return append(append([]ProcessStep{done[0]}, gap), recent...)
}

// Open reports whether the process ends at this vote's step with the business
// still going on: the step after it is not known, so none is shown.
func (p Process) Open() bool {
	open := false
	for _, s := range p.Steps {
		switch {
		case s.State == StepCurrent:
			open = true
		case s.State == StepNext || s.State.closes():
			return false
		}
	}
	return open
}

// Ended reports whether the vote ended the business.
func (p Process) Ended() bool {
	for _, s := range p.Steps {
		if s.State.closes() {
			return true
		}
	}
	return false
}

// Validate checks that p tells one coherent story: done steps, then the one
// step this vote took, then — unless that step ended the business or the next
// one is not known — the one step that comes next, and a next-step text
// wherever there is a step to come or the business closed.
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
		if s.State == StepSkipped {
			if current >= 0 || s.Skipped < 1 {
				return fmt.Errorf("step %d skips %d steps, after this vote's step: only steps taken can be skipped", i+1, s.Skipped)
			}
			continue
		}
		if strings.TrimSpace(s.Label) == "" {
			return fmt.Errorf("step %d has no label", i+1)
		}
		switch s.State {
		case StepDone:
			if current >= 0 {
				return fmt.Errorf("step %d is done but follows this vote's step", i+1)
			}
		case StepCurrent, StepEnded, StepCompleted:
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
	case p.Steps[current].State.closes() && next > 0:
		return errors.New("steps follow the step that ended the business")
	case p.Steps[current].State == StepCurrent && next > 1:
		return fmt.Errorf("an open business shows at most one next step, not %d", next)
	}
	if p.Steps[current].State.closes() && strings.TrimSpace(p.StatusLabel) == "" {
		return errors.New("a closed business needs the source's heading for its status")
	}
	text := strings.TrimSpace(p.Next)
	if p.Steps[current].State == StepCurrent && next == 0 {
		// The step after this vote is not known: no next step, or a line
		// that holds whichever it is, under a heading of its own.
		if text != "" && strings.TrimSpace(p.StatusLabel) == "" {
			return errors.New("a line in place of the next step needs its own heading")
		}
	} else if text == "" {
		return errors.New("a process needs a next step, or the source's word for a closed business")
	}
	if n := len([]rune(text)); n > MaxNextRunes {
		return fmt.Errorf("next step is %d runes, more than %d", n, MaxNextRunes)
	}
	return nil
}

// ProcessSource is implemented by a Source that can say where a group's
// business stands. For a business it has no rule for, or cannot read with
// certainty, Process returns a NoProcess saying why; any other error is a
// source it could not reach. Either way the group posts without a process —
// see voteposting.PrepareVoteGroups.
type ProcessSource interface {
	Process(group []Vote) (*Process, error)
}

// NoProcess is a ProcessSource's answer for a group it says no process for,
// and why: "Gesamtabstimmung: nächster Status nicht eindeutig". It is an
// expected answer, not a failure; the reason is there for the logs and for
// whoever checks which posts went out without one.
type NoProcess struct{ Reason string }

func (n NoProcess) Error() string { return "kein Prozess: " + n.Reason }

// Equal reports whether p and q describe the same process.
func (p *Process) Equal(q *Process) bool {
	if p == nil || q == nil {
		return p == q
	}
	if p.Next != q.Next || p.StatusLabel != q.StatusLabel || p.Kind != q.Kind || p.Bill != q.Bill || p.About != q.About || p.VoteAbout != q.VoteAbout || len(p.Steps) != len(q.Steps) {
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
