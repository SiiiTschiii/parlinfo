package platforms

// NoCard marks a thread post that carries no carousel card.
const NoCard = -1

// ThreadCards maps a vote thread onto the group's carousel: for each post, the
// index of the card it carries, or NoCard.
//
// The root carries the first card — the combined card of a lone vote, or the
// overview of a multi-vote group — which shows what the root says: the title
// and the verdicts. In a multi-vote group each vote's result card goes on the
// reply that opens that vote, given by voteStarts (reply indices, the root not
// counted). A lone vote's replies carry nothing, its one card being on the
// root already.
//
// The indices follow imagegen.GenerateCards: one combined card for a lone vote,
// otherwise the overview followed by one result card per vote.
func ThreadCards(groupSize, posts int, voteStarts []int) []int {
	out := make([]int, posts)
	for i := range out {
		out[i] = NoCard
	}
	if posts == 0 {
		return out
	}
	out[0] = 0
	if groupSize < 2 {
		return out
	}
	for v, reply := range voteStarts {
		if post := 1 + reply; post < posts {
			out[post] = v + 1
		}
	}
	return out
}
