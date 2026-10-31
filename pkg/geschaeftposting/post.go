package geschaeftposting

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
)

// beginnLayout is how PARIS writes Beginn.Start.
const beginnLayout = "2006-01-02T15:04:05.000"

// Filed returns the day g was filed, and false when PARIS gave no usable date.
func Filed(g zurichapi.Geschaeft) (time.Time, bool) {
	t, err := time.Parse(beginnLayout, g.Beginn.Start)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// Unposted returns the supported Geschäfte filed on or after cutoff that log
// does not hold, oldest first.
//
// Oldest first so a backlog drains in the order the council received it, and
// so a per-run budget that stops part-way leaves the newest for the next run
// rather than the oldest forever. Within one sitting the GR number breaks the
// tie, which is the order the Vorstösse were registered in.
//
// A Geschäft with no parseable filing date is skipped: there is no telling
// whether it is new, and announcing an old one as new is the worse mistake.
func Unposted(gs []zurichapi.Geschaeft, log *votelog.VoteLog, cutoff time.Time) []zurichapi.Geschaeft {
	seen := make(map[string]bool)
	var out []zurichapi.Geschaeft
	for _, g := range gs {
		if !Supported(g) || g.OBJGUID == "" || seen[g.OBJGUID] || log.IsPosted(g.OBJGUID) {
			continue
		}
		filed, ok := Filed(g)
		if !ok || filed.Before(cutoff) {
			continue
		}
		seen[g.OBJGUID] = true
		out = append(out, g)
	}

	sort.SliceStable(out, func(i, j int) bool {
		ti, _ := Filed(out[i])
		tj, _ := Filed(out[j])
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return grSortKey(out[i].GRNr) < grSortKey(out[j].GRNr)
	})
	return out
}

// grSortKey orders "2026/97" before "2026/541", which a string comparison
// would not. An unparseable number sorts last.
func grSortKey(grNr string) int {
	year, num, ok := strings.Cut(grNr, "/")
	y, err1 := strconv.Atoi(year)
	n, err2 := strconv.Atoi(num)
	if !ok || err1 != nil || err2 != nil {
		return int(^uint(0) >> 1)
	}
	return y*100000 + n
}

// Seed marks every one of gs as posted without posting anything, and saves.
//
// It is what a platform's first run does. The log starts empty, and an empty
// log reads as "nothing announced yet" — so without seeding, the first run
// would announce every Vorstoss inside the age window at once, most of them
// weeks old.
func Seed(gs []zurichapi.Geschaeft, log *votelog.VoteLog) error {
	for _, g := range gs {
		// No ref: nothing was published, and an empty ref is what the log
		// format uses for exactly that.
		log.MarkAsPosted(g.OBJGUID, votelog.PostRef{})
	}
	return log.Save()
}

// Formatter renders one Geschäft for one platform.
type Formatter func(g zurichapi.Geschaeft) (platforms.Content, error)

// Poster publishes formatted content. The vote platforms satisfy it, so a
// Geschäft goes out through exactly the code a vote does.
type Poster interface {
	// Post returns the published root, recorded in the log so an entry says
	// where its announcement went, and shouldContinue=false once the per-run
	// budget is spent.
	Post(content platforms.Content) (post votelog.PostRef, shouldContinue bool, err error)
	MaxPostsPerRun() int
	Name() string
}

// Post formats and publishes gs in order, marking each in log as soon as it is
// out and saving after every one, so a failure part-way through cannot cause
// what already went out to be posted again.
//
// In a dry run nothing is posted or recorded; the content is printed instead,
// under the same per-run budget a real run would stop at.
func Post(gs []zurichapi.Geschaeft, format Formatter, p Poster, log *votelog.VoteLog, dryRun bool) (int, error) {
	// The budget is otherwise only checked after a post, so a budget of 0 —
	// the way to pause one platform — would still publish one per run.
	if p.MaxPostsPerRun() <= 0 {
		fmt.Printf("⏸️  %s's per-run limit is %d; posting nothing\n", p.Name(), p.MaxPostsPerRun())
		return 0, nil
	}
	posted := 0
	for _, g := range gs {
		content, err := format(g)
		if err != nil {
			return posted, fmt.Errorf("formatting %s: %w", g.GRNr, err)
		}

		if dryRun {
			fmt.Println("────────────────────────────────────────────────────────────────────────────────")
			fmt.Printf("  %s %s (%s) [not visible in post]\n", g.Geschaeftsart, g.GRNr, g.Beginn.Text)
			fmt.Println("────────────────────────────────────────────────────────────────────────────────")
			fmt.Println(content.String())
			fmt.Println()
			posted++
			if posted >= p.MaxPostsPerRun() {
				break
			}
			continue
		}

		fmt.Printf("%s %s (%s):\n   %s\n", g.Geschaeftsart, g.GRNr, g.Beginn.Text, Link(g))
		ref, shouldContinue, err := p.Post(content)
		if err != nil {
			return posted, err
		}
		log.MarkAsPosted(g.OBJGUID, ref)
		if err := log.Save(); err != nil {
			return posted, err
		}
		posted++
		if !shouldContinue {
			fmt.Printf("⏸️  Reached %s's per-run limit (%d)\n", p.Name(), p.MaxPostsPerRun())
			break
		}
	}
	return posted, nil
}
