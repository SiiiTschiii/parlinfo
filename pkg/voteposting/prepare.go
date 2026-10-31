package voteposting

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// ErrUnsupportedVoteType is returned when a group contains a vote whose type or
// count format we do not recognise. The vote is skipped (not posted, not
// logged) and the run ends non-zero, because an unfamiliar type means the
// source is serving something new and somebody should look.
var ErrUnsupportedVoteType = errors.New("unsupported vote type")

// ErrUnpostableVoteType is returned for a vote whose type we recognise and have
// decided not to publish, such as an attendance roll call. The vote is skipped
// exactly as above, but the run still succeeds: these arrive on every sitting
// day and skipping them is the intended behaviour, not a fault to report.
var ErrUnpostableVoteType = errors.New("vote type not published")

// ErrInconsistentDecision is returned for a vote whose Decision field
// contradicts its raw counts (e.g. the source says "Ja" but Nein > Ja). The
// vote is skipped rather than posted — publishing a verdict the counts do not
// support is the one outcome this pipeline must never produce — and the run
// ends non-zero so the failure reaches somebody.
var ErrInconsistentDecision = errors.New("decision contradicts vote counts")

// PrepareVoteGroups fetches recent votes from a source, drops the ones already
// posted, and groups the rest for posting. It is platform-agnostic — every
// platform gets the same preparation.
//
// maxAgeDays limits how old a vote's sitting date can be (0 = no limit). It is
// not a freshness knob: it is the backstop that stops a re-indexed old vote
// being posted a second time when it falls outside what the vote log covers.
func PrepareVoteGroups(
	src votes.Source,
	voteLog *votelog.VoteLog,
	maxVotesToFetch int,
	maxAgeDays int,
) ([][]votes.Vote, error) {
	fetched, err := src.FetchRecent(maxVotesToFetch)
	if err != nil {
		return nil, err
	}

	if len(fetched) == 0 {
		return nil, nil
	}

	// Filter out already posted votes BEFORE grouping — cheaper, and grouping
	// can trigger further source calls.
	unposted := filterUnpostedVotes(fetched, voteLog)
	unposted = filterOldVotes(unposted, maxAgeDays)

	if len(unposted) == 0 {
		return nil, nil
	}

	groups, err := src.GroupByAffair(unposted)
	if err != nil {
		return nil, err
	}

	applyCompletenessGate(groups)

	return groups, nil
}

// applyCompletenessGate drops the member list from any vote whose recorded
// members do not account for the totals the source reported.
//
// A partial member list still renders a plausible-looking Fraktion table — it
// just understates whichever factions are missing, silently and unfalsifiably.
// Dropping it degrades the post to totals only, which are reported
// independently and remain correct.
//
// This lives here rather than in the adapters so every source is held to the
// same bar, and here rather than in the formatters so the decision is made once
// instead of once per platform.
func applyCompletenessGate(groups [][]votes.Vote) {
	for _, group := range groups {
		for i := range group {
			v := &group[i]
			if votes.IsBreakdownComplete(*v) {
				continue
			}
			total, _ := v.TotalRecorded()
			log.Printf("⚠️  %s: %d member votes for reported totals of %d — posting totals only, without the Fraktion breakdown",
				v.SourceID, len(v.MemberVotes), total)
			v.MemberVotes = nil
		}
	}
}

// filterUnpostedVotes filters out votes that have already been posted.
func filterUnpostedVotes(vs []votes.Vote, voteLog *votelog.VoteLog) []votes.Vote {
	var unposted []votes.Vote
	for _, v := range vs {
		if !voteLog.IsPosted(v.SourceID) {
			unposted = append(unposted, v)
		}
	}
	return unposted
}

// filterOldVotes drops votes whose sitting date is more than maxAgeDays old.
// Votes with an unknown date are kept: an unparseable date is a data problem,
// and silently dropping a vote is worse than posting one that is a little old.
func filterOldVotes(vs []votes.Vote, maxAgeDays int) []votes.Vote {
	if maxAgeDays <= 0 {
		return vs
	}
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	var recent []votes.Vote
	for _, v := range vs {
		if !v.Date.IsZero() && v.Date.Before(cutoff) {
			log.Printf("⚠️  Skipping old vote %s (date %s, older than %d days)",
				v.SourceID, v.DateString(), maxAgeDays)
			continue
		}
		recent = append(recent, v)
	}
	return recent
}

// DropBefore drops the groups whose sitting date falls before from, a calendar
// date. It is how a platform that goes live late stays clear of the history it
// never posted, without a vote log that claims it did: the log records what was
// posted, and nothing else may write to it.
//
// Dates are compared as calendar days in the zone the source reported them in,
// so a sitting at 00:00 Zurich time on from's own day is kept rather than lost
// to a UTC offset. A group whose date is unknown is kept, as filterOldVotes
// keeps such votes: a data problem should not decide what is published, and the
// next run sees the same group again.
func DropBefore(groups [][]votes.Vote, from time.Time) [][]votes.Vote {
	if from.IsZero() {
		return groups
	}
	var kept [][]votes.Vote
	for _, g := range groups {
		d := groupDate(g)
		day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
		if day.Before(from) {
			continue
		}
		kept = append(kept, g)
	}
	return kept
}

// VoteLogs resolves the log a group belongs in, keyed by jurisdiction. A
// channel serving several jurisdictions posts them through one platform
// instance but must record each in its own log.
type VoteLogs map[string]*votelog.VoteLog

// SingleLog builds a VoteLogs for the common one-jurisdiction case.
func SingleLog(jurisdiction string, vl *votelog.VoteLog) VoteLogs {
	return VoteLogs{jurisdiction: vl}
}

func (l VoteLogs) forGroup(group []votes.Vote) (*votelog.VoteLog, error) {
	key := group[0].Jurisdiction
	vl, ok := l[key]
	if !ok {
		return nil, fmt.Errorf("no vote log configured for jurisdiction %q", key)
	}
	return vl, nil
}

// MergeOldestFirst interleaves groups from several jurisdictions into the order
// they should be posted: oldest vote first, ties broken by the order the
// jurisdictions were passed in.
//
// This is what stops a busy sitting in one chamber from starving the other when
// they share a per-run budget, and it preserves today's behaviour of draining a
// backlog chronologically.
func MergeOldestFirst(perJurisdiction ...[][]votes.Vote) [][]votes.Vote {
	var merged [][]votes.Vote
	for _, groups := range perJurisdiction {
		merged = append(merged, groups...)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return groupDate(merged[i]).Before(groupDate(merged[j]))
	})
	return merged
}

// groupDate is the earliest known vote date in a group. Groups whose dates are
// all unknown sort last, so a data problem cannot jump the queue.
func groupDate(group []votes.Vote) time.Time {
	var earliest time.Time
	for _, v := range group {
		if v.Date.IsZero() {
			continue
		}
		if earliest.IsZero() || v.Date.Before(earliest) {
			earliest = v.Date
		}
	}
	if earliest.IsZero() {
		return time.Unix(1<<62, 0)
	}
	return earliest
}

// PostToPlatform posts vote groups to a platform.
// If dryRun is true, only prints the content without posting.
// Returns the number of groups successfully posted.
//
// The platform instance carries the per-run budget, so callers must pass the
// same instance for every jurisdiction on a channel — constructing one per
// jurisdiction resets the counter and doubles what the account posts.
func PostToPlatform(
	groups [][]votes.Vote,
	platform platforms.Platform,
	voteLogs VoteLogs,
	dryRun bool,
) (int, error) {
	posted := 0

	var firstRejection error

	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		// Drop votes no formatter can render, keeping the rest of the group.
		group := postableVotes(group, &firstRejection)
		if len(group) == 0 {
			continue
		}

		// Format the content
		content, err := platform.Format(group)
		if err != nil {
			return posted, err
		}

		if dryRun {
			// Dry run: print and respect the same per-run limit as real posting
			if posted > 0 {
				fmt.Println()
			}
			fmt.Println("────────────────────────────────────────────────────────────────────────────────")
			fmt.Printf("  📋 %s %s (%s) — %d vote(s) [not visible in post]\n",
				group[0].Jurisdiction,
				group[0].Affair.Number,
				group[0].DateString(),
				len(group),
			)
			fmt.Println("────────────────────────────────────────────────────────────────────────────────")
			fmt.Println()
			fmt.Println(content.String())
			posted++
			if posted >= platform.MaxPostsPerRun() {
				break
			}
		} else {
			voteLog, err := voteLogs.forGroup(group)
			if err != nil {
				return posted, err
			}

			// Real posting — log which group for tracing
			fmt.Printf("📋 %s %s (%s) — %d vote(s):\n",
				group[0].Jurisdiction,
				group[0].Affair.Number,
				group[0].DateString(),
				len(group),
			)
			for _, v := range group {
				fmt.Printf("   %s\n", v.SourceURL)
			}

			post, shouldContinue, err := platform.Post(content)
			if err != nil {
				return posted, err
			}

			// Mark the posted votes. Any vote dropped above is deliberately
			// absent from this slice, so it is retried rather than buried.
			for _, v := range group {
				voteLog.MarkAsPosted(v.SourceID, post)
			}

			// Save vote log after each successful post
			if err := voteLog.Save(); err != nil {
				return posted, err
			}

			posted++

			// Check if we should stop
			if !shouldContinue {
				break
			}
		}
	}

	if firstRejection != nil {
		return posted, firstRejection
	}
	return posted, nil
}

// postableVotes returns the votes of a group the formatters can render, and
// records the first rejection in firstErr so the run still ends non-zero.
//
// Rejection is per vote rather than per group, which matters more than it
// sounds. Votes are grouped by business matter and sitting day, and Kanton
// Zürich interleaves vote types freely within one business: the 15.12.2025
// Steuerfuss item carries five ordinary Ja/Nein votes alongside three
// Cup-Abstimmung rounds, and the 19.01.2026 Lehrpersonalgesetz item five
// alongside one. Failing the whole group would have suppressed the tax-rate
// decision entirely because three procedural rounds in the same business are
// unrenderable — silently trading one unpublishable vote for five publishable
// ones.
//
// The dropped vote is not logged as posted, so it returns on the next run and
// keeps failing visibly until the source is fixed or it ages out of the window.
func postableVotes(group []votes.Vote, firstErr *error) []votes.Vote {
	postable := make([]votes.Vote, 0, len(group))
	for _, v := range group {
		if err := validateVote(v); err != nil {
			log.Printf("⚠️  Skipping vote: %v", err)
			// A type we know and have chosen not to publish is routine —
			// every sitting opens with an attendance roll call — and failing
			// on those would leave the run permanently red while the bot
			// behaves exactly as intended. The other two rejections mean the
			// source is serving something we cannot vouch for, and nobody
			// finds out unless the run reports it.
			if !errors.Is(err, ErrUnpostableVoteType) && *firstErr == nil {
				*firstErr = err
			}
			continue
		}
		postable = append(postable, v)
	}
	return withoutMixedRollCalls(postable)
}

// withoutMixedRollCalls drops election roll calls from a group that also holds
// real votes.
//
// A roll call is posted only as a Wahlgeschäft notice, and a notice needs the
// whole group to be roll calls — see votes.IsWahlgeschaeftGroup. Beside a real
// vote it would take the ordinary path instead, and its headcount would be
// rendered next to an actual result as if it were one. The real votes carry
// counts that mean something, so they are what the post keeps.
func withoutMixedRollCalls(group []votes.Vote) []votes.Vote {
	if len(group) == 0 || votes.IsWahlgeschaeftGroup(group) {
		return group
	}
	kept := make([]votes.Vote, 0, len(group))
	for _, v := range group {
		if votes.IsWahlgeschaeft(v) {
			log.Printf("⚠️  Skipping vote: %s is an election roll call in a group with real votes, so it is not posted", v.SourceID)
			continue
		}
		kept = append(kept, v)
	}
	return kept
}

// IsRejectedVoteError reports whether an error is the pipeline refusing to
// publish a vote, as opposed to a platform failing to post one. The two need
// different words in the log: the first means the run did its job on everything
// else, the second means it did not.
func IsRejectedVoteError(err error) bool {
	return errors.Is(err, ErrUnsupportedVoteType) || errors.Is(err, ErrInconsistentDecision)
}

// PostableGroups applies the posting filter without posting, so a preview shows
// what the bot would publish rather than everything the source served. Without
// it a preview renders the attendance roll calls that posting refuses — the
// very votes this pipeline exists to keep off the timeline.
func PostableGroups(groups [][]votes.Vote) [][]votes.Vote {
	var ignored error
	out := make([][]votes.Vote, 0, len(groups))
	for _, group := range groups {
		if postable := postableVotes(group, &ignored); len(postable) > 0 {
			out = append(out, postable)
		}
	}
	return out
}

// validateVote checks that one vote is something this pipeline can publish: a
// type on the handled list, a recognisable count format (standard Ja/Nein or
// Auswahl A-E), and a stated decision its own counts support.
//
// The type check is what catches a source starting to serve something new. The
// count check alone would not: an attendance determination and a quorum vote
// both look like a perfectly ordinary lopsided Ja/Nein tally, and would post
// happily while saying something untrue about how parliament voted.
//
// The decision check catches the other direction, a type we render correctly
// carrying a verdict the source got wrong — PARIS reported the 02.09.2026
// Zuweisung on 2026/408 as carried at 56 Ja to 58 Nein, and corrected it days
// later. Rendering that would have published the opposite of what the council
// decided.
func validateVote(v votes.Vote) error {
	// An election business is an Anwesenheitsermittlung that
	// IsKnownUnpostableType would otherwise reject. It gets a post that
	// reports no result instead of the usual Ja/Nein rendering, because the
	// roll call is the only trace an election leaves — see
	// votes.IsWahlgeschaeft and voteformat.WahlgeschaeftBody.
	if votes.IsWahlgeschaeft(v) {
		return nil
	}
	if voteformat.IsKnownUnpostableType(v.Type) {
		return withVoteLink(v, "%w: vote %s (%q, %s) has type %q, which is not published",
			ErrUnpostableVoteType, v.SourceID, voteDescription(v), countsSummary(v), v.Type)
	}
	if !voteformat.IsHandledVoteType(v.Type) {
		return withVoteLink(v, "%w: vote %s (%q, %s) has type %q, which no formatter handles",
			ErrUnsupportedVoteType, v.SourceID, voteDescription(v), countsSummary(v), v.Type)
	}
	// A quorum round nobody voted in is an attempt the chamber repeats, not a
	// format we fail to read: on 2026-09-28 the Kantonsrat opened three
	// rounds within a minute and the first recorded 0 Ja. The round that
	// counts arrives as its own vote and is posted.
	if voteformat.IsEmptyQuorumRound(voteformat.CountsOf(v)) {
		return withVoteLink(v, "%w: vote %s (%q, type=%q) is a quorum round with no votes cast",
			ErrUnpostableVoteType, v.SourceID, v.Subtitle, v.Type)
	}
	if voteformat.IsUnsupportedVoteType(voteformat.CountsOf(v)) {
		return withVoteLink(v, "%w: vote %s (%q, type=%q) has all-zero counts",
			ErrUnsupportedVoteType, v.SourceID, v.Subtitle, v.Type)
	}
	// Last, so that a vote we would not publish anyway is reported as the
	// routine skip it is rather than as a data fault that reddens the run.
	if !voteformat.IsDecisionConsistent(v.Decision, v.Yes, v.No) {
		return withVoteLink(v, "%w: %s %q (%s) has Decision=%q but Ja=%d Nein=%d",
			ErrInconsistentDecision,
			v.Affair.Number,
			voteformat.CleanVoteSubtitle(voteDescription(v)),
			voteformat.CleanVoteTitle(v.Affair.Title),
			v.Decision, *v.Yes, *v.No,
		)
	}
	return nil
}

// withVoteLink formats a rejection and hangs the source's own page for the vote
// underneath it.
//
// Every rejection here is read by somebody who then has to decide whether the
// source is wrong or we are, and the only way to decide is to look at the vote.
// Identifying it from a business number and a title used to take three API calls
// by hand. Sources that publish no per-vote page get the message alone.
func withVoteLink(v votes.Vote, format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	if v.SourceURL == "" {
		return err
	}
	return fmt.Errorf("%w\n   %s", err, v.SourceURL)
}

// voteDescription names the vote for a log line, preferring whichever of the
// two title fields carries something. A bare id and an empty subtitle — which
// is what a skipped Kanton Zürich vote used to log — say nothing about what was
// skipped, and identifying it then meant three API calls by hand.
func voteDescription(v votes.Vote) string {
	if v.Subtitle != "" {
		return v.Subtitle
	}
	return v.Title
}

// countsSummary renders the tally compactly, so a reader of the log can tell a
// roll call from a contested vote at a glance.
func countsSummary(v votes.Vote) string {
	n := func(p *int) string {
		if p == nil {
			return "–"
		}
		return strconv.Itoa(*p)
	}
	return fmt.Sprintf("%s/%s/%s/%s Ja/Nein/Enth/Abw",
		n(v.Yes), n(v.No), n(v.Abstention), n(v.Absent))
}

// LatestGroups returns what the bot would post for a source right now, newest
// first: at most numGroups groups (0 = all) from the latest fetchLimit votes.
// It is for the tools that preview or test-post live votes.
//
// The vote log is empty and the age guard off, because previewing is exactly
// when already-posted and out-of-window votes still need to show up. Groups
// posting would refuse are dropped, as PostableGroups explains.
//
// Newest first, because a preview is nearly always about the most recent
// sitting, where PrepareVoteGroups orders oldest first so the bot drains a
// backlog chronologically.
func LatestGroups(src votes.Source, jurisdictionKey string, fetchLimit, numGroups int) ([][]votes.Vote, error) {
	emptyLog := votelog.NewEmpty(jurisdictionKey, votelog.PlatformInstagram)
	groups, err := PrepareVoteGroups(src, emptyLog, fetchLimit, 0)
	if err != nil {
		return nil, err
	}
	groups = PostableGroups(groups)
	for i, j := 0, len(groups)-1; i < j; i, j = i+1, j-1 {
		groups[i], groups[j] = groups[j], groups[i]
	}
	if numGroups > 0 && len(groups) > numGroups {
		groups = groups[:numGroups]
	}
	return groups, nil
}
