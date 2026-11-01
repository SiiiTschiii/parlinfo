package bluesky

import (
	"fmt"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/bskyapi"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// maxGraphemes is the Bluesky post character limit (graphemes)
const maxGraphemes = 300

// BlueskyPost holds the formatted text and rich text facets for a Bluesky post
type BlueskyPost struct {
	Text     string
	Facets   []bskyapi.Facet
	Mentions []contacts.BlueskyMention // unresolved mentions (handle + byte offsets)
	Images   []imagegen.Card           // carousel cards to embed, at most bskyapi.MaxImagesPerPost
}

// FormatVoteThread creates a Bluesky thread for a group of related votes.
// Returns a slice of posts: [0] is the root post, [1:] are replies.
//
// Root post contains: header, title, result (single vote), thread hint
// Replies contain: the links, then vote details (counts per vote)
func FormatVoteThread(group []votes.Vote, contactMapper *contacts.Mapper) []*BlueskyPost {
	thread, _ := FormatVoteThreadWithCards(group, contactMapper)
	return thread
}

// FormatVoteThreadWithCards is FormatVoteThread, also returning for each post
// the index of the carousel card it carries, or platforms.NoCard. See
// platforms.ThreadCards.
func FormatVoteThreadWithCards(group []votes.Vote, contactMapper *contacts.Mapper) ([]*BlueskyPost, []int) {
	if len(group) == 0 {
		return nil, nil
	}

	// A Wahlgeschäft gets one notice, not a thread: its only votes are
	// attendance roll calls, so there are no counts to put in replies. This
	// must run before anything below touches voteformat.CountsOf,
	// FormatVoteCounts* or a verdict emoji, none of which mean anything for a
	// headcount. It is checked on the whole group because a second ballot
	// round adds a second roll call to the same business.
	// No mentions are resolved on this path, deliberately: the only name in
	// the text belongs to the member being replaced, not to anyone elected.
	if votes.IsWahlgeschaeftGroup(group) {
		posts := buildWahlgeschaeftPosts(group)
		return posts, platforms.ThreadCards(1, len(posts), nil)
	}

	firstVote := group[0]

	// Common components
	title := voteformat.CleanVoteTitle(firstVote.Title)

	// --- Build root post ---
	root, deferred := buildRootPost(group, title)

	// --- Build reply posts ---
	// Signatories the root had no room for lead the thread: naming them costs a
	// line here, where truncating the title to keep them would have cost the
	// subject of the vote.
	replies, voteStarts := buildReplyPosts(group, voteformat.SignatoryLine(group, deferred),
		voteformat.LinkLine(group), linkURLs(group))

	thread := make([]*BlueskyPost, 0, 1+len(replies))
	thread = append(thread, root)
	thread = append(thread, replies...)

	// Scan all posts for politician mentions with Bluesky accounts
	if contactMapper != nil {
		for _, post := range thread {
			post.Mentions = mergeMentions(contactMapper.FindBlueskyMentions(post.Text),
				contactMapper.FindBlueskyFraktionMentions(post.Text, group[0].Jurisdiction))
		}
	}

	return thread, platforms.ThreadCards(len(group), len(thread), voteStarts)
}

// replyEntry is one block of reply text: a link block (link) or the signatory
// line, both with vote -1, or a vote's counts (starts) or its Fraktion
// breakdown.
type replyEntry struct {
	text   string
	vote   int
	starts bool
	link   bool
}

// buildStilleWahlPosts announces a stille Wahl: the office and who was elected
// to it, and the links — no counts, no verdict emoji, no "Details im Thread".
// See voteformat.AsStilleWahl/StilleWahlBody for why.
//
// It stays one post wherever the links fit on it, which is what a Wahlgeschäft
// notice should be: there is nothing to put in a thread. Where they do not — Kanton
// Zürich carries three links plus the licence credit, some 380 graphemes
// against a 300 limit — they spill into replies rather than the post going to
// the API at a length it rejects.
func buildWahlgeschaeftPosts(group []votes.Vote) []*BlueskyPost {
	header := fmt.Sprintf("🗳️ %s\n\n", voteformat.PostHeadline(group))
	link := voteformat.LinkLine(group)
	body := voteformat.WahlgeschaeftBody(group)
	urls := linkURLs(group)

	if graphemeLen(header+body+link) <= maxGraphemes {
		return []*BlueskyPost{makePost(header+body+link, urls...)}
	}

	// The body is budgeted against the header alone, because the links are no
	// longer riding on this post. Within it the title gives way and the
	// sentence does not: an election post that loses the sentence says nothing
	// about why it reports no result. truncateText appends its own "…", so
	// that comes out of the budget too.
	sentence := voteformat.WahlgeschaeftNoResult
	available := maxGraphemes - graphemeLen(header+sentence+"\n\n") - graphemeLen("…")
	if available > 0 {
		body = truncateText(voteformat.CleanVoteTitle(group[0].Title), available) + "\n\n" + sentence
	} else {
		body = sentence
	}

	posts := []*BlueskyPost{makePost(header + body)}
	for _, chunk := range linkChunks(link) {
		posts = append(posts, makePost(chunk, urls...))
	}
	return posts
}

// buildRootPost creates the root post with header, title, result, and thread hint.
// If the title is too long, it is truncated with "…"; replies go straight to vote details.
func buildRootPost(group []votes.Vote, title string) (*BlueskyPost, []votes.Author) {
	header := fmt.Sprintf("🗳️ %s\n\n", voteformat.PostHeadline(group))
	threadHint := "\n\n👇 Details im Thread"

	// The label line: what kind of business this is, who filed it, plus the
	// Abstimmungsgegenstand when a lone vote makes that meaningful.
	//
	// Sized against what the title leaves, so a long signatory list sheds names
	// into the thread rather than eating the subject of the vote.
	body := title
	if len(group) == 1 && voteformat.HasVerdict(group[0]) {
		body = fmt.Sprintf("%s %s: %s", voteformat.GetVoteResultEmoji(group[0].Decision),
			voteformat.GetVoteResultText(group[0].Decision), title)
	}
	available := maxGraphemes - graphemeLen(header) - graphemeLen(threadHint) - graphemeLen(body) - 1
	subtitlePrefix, deferred := voteformat.FitAuthorPrefix(group, available, graphemeLen)

	if subtitlePrefix != "" {
		body = subtitlePrefix + "\n" + body
	}

	fullText := header + body + threadHint

	// Truncate title if root exceeds limit (rare, only for very long titles)
	if graphemeLen(fullText) > maxGraphemes {
		overhead := graphemeLen(header) + graphemeLen(threadHint) + 1 // 1 for "…"
		if subtitlePrefix != "" {
			overhead += graphemeLen(subtitlePrefix) + 1 // +1 for "\n"
		}
		titleRoom := maxGraphemes - overhead
		if len(group) == 1 {
			vote := group[0]
			if !voteformat.HasVerdict(vote) {
				title = truncateText(title, titleRoom)
				body = title
			} else {
				// Truncate after "✅ Angenommen: " prefix
				resultEmoji := voteformat.GetVoteResultEmoji(vote.Decision)
				result := voteformat.GetVoteResultText(vote.Decision)
				prefix := fmt.Sprintf("%s %s: ", resultEmoji, result)
				titleAvailable := titleRoom - graphemeLen(prefix)
				if titleAvailable > 0 {
					title = truncateText(title, titleAvailable)
				}
				body = prefix + title
			}
		} else {
			body = truncateText(title, titleRoom)
		}
		// Reattached for both shapes: the overhead above reserves room for it, so
		// dropping it here shortened the post and lost the line at once.
		if subtitlePrefix != "" {
			body = subtitlePrefix + "\n" + body
		}
		fullText = header + body + threadHint
	}

	return &BlueskyPost{Text: fullText}, deferred
}

// buildReplyPosts creates reply posts with vote details and link.
// Packs entries into replies (≤300 graphemes).
// The link block leads, and each vote opens a reply of its own.
func buildReplyPosts(group []votes.Vote, signatoryLine, linkLine string, linkURLs []string) ([]*BlueskyPost, []int) {

	// Build individual vote entries
	var entries []replyEntry

	// The links lead the thread. They are about the whole business, not about
	// any one vote, so they sit where a reader meets them first rather than
	// after the last vote — but not on the root, which needs its room for the
	// title, and on Bluesky could not hold them at all.
	// linkChunks splits a block too long for one post between its lines.
	for _, chunk := range linkChunks(linkLine) {
		entries = append(entries, replyEntry{text: chunk, vote: -1, link: true})
	}
	// The signatories the root could not hold come next, so a reader meets
	// them before the tallies rather than after them.
	if signatoryLine != "" {
		entries = append(entries, replyEntry{text: signatoryLine, vote: -1})
	}

	for i, vote := range group {
		var entry strings.Builder

		counts := voteformat.CountsOf(vote)
		if len(group) == 1 {
			// Single vote: the counts, headed by the ballot type when it is one
			// worth naming. A lone threshold vote needs that most — there is no
			// sibling beside it to make the lopsided tally look unusual.
			if label := voteformat.TypeLabel(vote.Type); label != "" {
				entry.WriteString(label + "\n")
			}
			entry.WriteString(voteformat.FormatVoteCounts(counts))
		} else {
			// Multi-vote: subtitle + counts
			voteTitle := voteformat.SubVoteLabel(vote, i, len(group))
			if !voteformat.HasVerdict(vote) {
				// Auswahl: no ✅/❌ prefix
				entry.WriteString(fmt.Sprintf("%s\n", voteTitle))
			} else {
				voteEmoji := voteformat.GetVoteResultEmoji(vote.Decision)
				entry.WriteString(fmt.Sprintf("%s %s\n", voteEmoji, voteTitle))
			}
			entry.WriteString(voteformat.FormatVoteCounts(counts))
		}

		entries = append(entries, replyEntry{text: entry.String(), vote: i, starts: true})

		// Add Fraktion breakdown as separate entry
		if len(vote.MemberVotes) > 0 {
			fraktionCounts := voteformat.AggregateFraktionCounts(vote)
			if breakdown := voteformat.FormatFraktionBreakdown(fraktionCounts); breakdown != "" {
				entries = append(entries, replyEntry{text: breakdown, vote: i})
			}
		}
	}

	// Pack entries into replies, respecting the grapheme limit.
	// The last reply gets the link appended.
	var replies []*BlueskyPost
	var currentEntries []string
	currentLen := 0
	// Each vote opens a reply of its own, so the reply carrying a vote's card
	// talks about that vote and no other. What belongs to no vote — the links
	// and the signatories — may share the first vote's reply, except that in a
	// multi-vote group the links keep a reply to themselves: that first vote's
	// reply carries its card, and the links are not about that vote.
	multiVote := len(group) > 1
	voteStarts := make([]int, len(group))
	currentHasVote, currentHasLink := false, false
	flush := func() {
		if len(currentEntries) > 0 {
			replies = append(replies, makePost(strings.Join(currentEntries, "\n\n"), linkURLs...))
		}
		currentEntries, currentLen = nil, 0
		currentHasVote, currentHasLink = false, false
	}

	for _, e := range entries {
		entryLen := graphemeLen(e.text)
		if e.starts && (currentHasVote || (multiVote && currentHasLink)) {
			flush()
		}
		separatorLen := 0
		if len(currentEntries) > 0 {
			separatorLen = 2 // "\n\n" between entries
		}
		if currentLen+separatorLen+entryLen > maxGraphemes && len(currentEntries) > 0 {
			flush()
			separatorLen = 0
		}

		if e.starts {
			voteStarts[e.vote] = len(replies)
		}
		currentHasVote = currentHasVote || e.vote >= 0
		currentHasLink = currentHasLink || e.link
		currentEntries = append(currentEntries, e.text)
		currentLen += separatorLen + entryLen
	}
	flush()

	return replies, voteStarts
}

// linkChunks packs the trailing link block into as few posts as hold it,
// splitting only between lines.
//
// Bluesky counts graphemes where X charges a flat 23 per URL, so a link block
// that costs 128 characters of an X post costs nearly 380 of a Bluesky one —
// more than a post can hold. The block therefore has to be able to span
// replies, and it has to break between lines: a URL cut in half is a URL nobody
// can click, and the second half posts as plain text.
//
// A single line longer than the limit is returned as it stands. No source
// produces one — the longest link this posts is a recapp deep link at about 130
// graphemes with its label — and there is nothing truthful to do with it here
// anyway, since a URL cannot be shortened without breaking it.
func linkChunks(block string) []string {
	block = strings.Trim(block, "\n")
	if block == "" {
		return nil
	}

	var out []string
	current := ""
	for _, line := range strings.Split(block, "\n") {
		if current == "" {
			current = line
			continue
		}
		if graphemeLen(current+"\n"+line) > maxGraphemes {
			out = append(out, current)
			current = line
			continue
		}
		current += "\n" + line
	}
	return append(out, current)
}

// linkURLs is the URLs of the trailing link block, in render order, for
// faceting.
func linkURLs(group []votes.Vote) []string {
	links := voteformat.GroupLinks(group)
	out := make([]string, 0, len(links))
	for _, l := range links {
		out = append(out, l.URL)
	}
	return out
}

// makePost creates a BlueskyPost with a link facet per URL it carries.
func makePost(text string, links ...string) *BlueskyPost {
	return &BlueskyPost{Text: text, Facets: buildLinkFacets(text, links)}
}

// buildLinkFacets locates each URL in the text and creates a link facet for it.
//
// Bluesky renders no URL as a link unless a facet says so, so a post carrying
// three links needs three facets: miss one and that line publishes as plain
// text a reader has to copy by hand. Facets must also arrive in ascending byte
// order, which they do here because the links are searched in the order
// voteformat.GroupLinks rendered them.
//
// A URL the text does not contain is skipped rather than faceted at a wrong
// offset — a facet whose range does not cover its own URL would linkify some
// neighbouring stretch of the post.
func buildLinkFacets(text string, urls []string) []bskyapi.Facet {
	var facets []bskyapi.Facet
	for _, u := range urls {
		if u == "" {
			continue
		}
		idx := strings.Index(text, u)
		if idx < 0 {
			continue
		}
		facets = append(facets, bskyapi.LinkFacet(idx, idx+len(u), u))
	}
	return facets
}

// truncateText truncates a string to fit within maxRunes graphemes, adding "…".
func truncateText(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	truncated := strings.TrimRight(string(runes[:maxRunes]), " \n")
	return truncated + "…"
}

// graphemeLen returns the number of graphemes (runes) in a string.
func graphemeLen(s string) int {
	return len([]rune(s))
}

// mergeMentions adds the extra mentions that do not overlap one already found,
// so a name inside a Fraktion phrase is never given two facets.
func mergeMentions(found, extra []contacts.BlueskyMention) []contacts.BlueskyMention {
	for _, e := range extra {
		overlaps := false
		for _, f := range found {
			if e.ByteStart < f.ByteEnd && f.ByteStart < e.ByteEnd {
				overlaps = true
				break
			}
		}
		if !overlaps {
			found = append(found, e)
		}
	}
	return found
}
