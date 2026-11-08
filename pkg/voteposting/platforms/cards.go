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
// counted). A lone vote's replies carry nothing else, its one result card being
// on the root already.
//
// statusCard says the carousel has a status card (see imagegen.HasStatusCard).
// It goes on the first reply, which opens with the links and the next-step line
// the card illustrates, provided that reply carries no vote's card already —
// it never does while the group has links, which every source supplies. Where
// it would, the status card is left out of the thread rather than displacing a
// result.
//
// The indices follow imagegen.GenerateCards: the first card, the status card if
// there is one, then one result card per vote of a multi-vote group.
func ThreadCards(groupSize, posts int, voteStarts []int, statusCard bool) []int {
	out := make([]int, posts)
	for i := range out {
		out[i] = NoCard
	}
	if posts == 0 {
		return out
	}
	out[0] = 0
	first := 1
	if statusCard {
		first = 2
	}
	if groupSize >= 2 {
		for v, reply := range voteStarts {
			if post := 1 + reply; post < posts {
				out[post] = v + first
			}
		}
	}
	if statusCard && posts > 1 && out[1] == NoCard {
		out[1] = 1
	}
	return out
}
