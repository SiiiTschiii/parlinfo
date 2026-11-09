package facebook

import (
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
)

func TestFormatPost_EmptyVotes(t *testing.T) {
	if _, err := FormatPost(nil); err == nil {
		t.Fatal("expected error for an empty group")
	}
}

func TestFormatPost_AllFixtures(t *testing.T) {
	for name, group := range testfixtures.AllFixtures() {
		t.Run(name, func(t *testing.T) {
			content, err := FormatPost(group)
			if err != nil {
				t.Fatalf("FormatPost: %v", err)
			}
			if len(content.Images) == 0 || len(content.Images) > maxPhotos {
				t.Errorf("%d images, want 1 to %d", len(content.Images), maxPhotos)
			}
			if !strings.HasPrefix(content.Message, "🗳️ ") {
				t.Errorf("message should open with the headline, got %q", firstLine(content.Message))
			}
			if strings.HasSuffix(content.Message, "\n") {
				t.Errorf("message should not end in a newline")
			}
			// With photos attached there is no preview card, so the URLs are the
			// only way to the vote page: every one the other platforms carry has
			// to be in the text.
			for _, l := range voteformat.GroupLinks(group) {
				if !strings.Contains(content.Message, l.URL) {
					t.Errorf("message is missing link %s", l.URL)
				}
			}
		})
	}
}

// A single vote has one card and nothing to split, so its post keeps the whole
// Instagram caption body.
func TestFormatPost_SingleVoteKeepsAllTextInThePost(t *testing.T) {
	group := testfixtures.SingleVoteAngenommen()
	content, err := FormatPost(group)
	if err != nil {
		t.Fatalf("FormatPost: %v", err)
	}
	body := strings.TrimRight(voteformat.LongFormBody(group, voteformat.Tags{}), "\n")
	if !strings.HasPrefix(content.Message, body+"\n\n") {
		t.Errorf("message does not start with the long-form body:\n%s", content.Message)
	}
	for i, c := range content.Captions {
		if c != "" {
			t.Errorf("caption %d = %q, want none for a single card", i, c)
		}
	}
}

// A multi-vote post carries the heading and links; each vote's counts ride on
// its own card, after the title card.
func TestFormatPost_MultiVoteCaptionsEachCard(t *testing.T) {
	group := testfixtures.MultiVoteGroup()
	content, err := FormatPost(group)
	if err != nil {
		t.Fatalf("FormatPost: %v", err)
	}
	if len(content.Images) != len(group)+1 || len(content.Captions) != len(content.Images) {
		t.Fatalf("%d images and %d captions for %d votes, want one card per vote plus a title card",
			len(content.Images), len(content.Captions), len(group))
	}
	if content.Captions[0] != "" {
		t.Errorf("the title card should have no caption, got %q", content.Captions[0])
	}
	for i := range group {
		want := voteBlock(group, i)
		if got := content.Captions[i+1]; got != want {
			t.Errorf("card %d caption:\n%s\nwant:\n%s", i+1, got, want)
		}
		// The point of the split: the counts are not repeated in the post.
		if strings.Contains(content.Message, voteformat.FormatVoteCountsLong(voteformat.CountsOf(group[i]))) {
			t.Errorf("vote %d's counts are in the post text as well as its caption", i)
		}
	}
	if !strings.HasPrefix(content.Message, strings.TrimRight(voteformat.LongFormHeading(group, voteformat.Tags{}), "\n")) {
		t.Errorf("message does not start with the heading:\n%s", content.Message)
	}
}

// With more votes than cards, the votes past the cap have nowhere to caption.
// Their text goes in the post: nothing is lost to the cap.
func TestFormatPost_VotesWithoutACardStayInThePost(t *testing.T) {
	group := testfixtures.TenVoteStressTest()
	content, err := FormatPost(group)
	if err != nil {
		t.Fatalf("FormatPost: %v", err)
	}
	if len(content.Images) != maxPhotos {
		t.Fatalf("%d images, want the cap of %d", len(content.Images), maxPhotos)
	}
	for i := range group {
		block := voteBlock(group, i)
		inCaption := i+1 < len(content.Captions) && content.Captions[i+1] == block
		inMessage := strings.Contains(content.Message, block)
		if inCaption == inMessage {
			t.Errorf("vote %d is in the caption=%v and the message=%v, want exactly one", i, inCaption, inMessage)
		}
	}
}

// The Instagram fixture built to overflow 2,200 characters posts whole here.
func TestFormatPost_LongGroupIsNotTruncated(t *testing.T) {
	group := testfixtures.InstagramLongMultiVoteTruncation()
	content, err := FormatPost(group)
	if err != nil {
		t.Fatalf("FormatPost: %v", err)
	}
	if strings.Contains(content.Message, "Gekürzt") {
		t.Errorf("long group was truncated")
	}
	if n := len([]rune(content.Message)); n <= 2200 {
		t.Errorf("fixture expected to exceed Instagram's 2200 characters, got %d", n)
	}
	if len(content.Images) > maxPhotos {
		t.Errorf("%d images, over the cap of %d", len(content.Images), maxPhotos)
	}
}

func TestFormatPost_WahlgeschaeftHasNoCounts(t *testing.T) {
	content, err := FormatPost(testfixtures.KantonsratWahlgeschaeft())
	if err != nil {
		t.Fatalf("FormatPost: %v", err)
	}
	if !strings.Contains(content.Message, voteformat.WahlgeschaeftNoResult) {
		t.Errorf("Wahlgeschäft notice is missing its no-result sentence:\n%s", content.Message)
	}
	for _, marker := range []string{"✅", "❌", "Ja", "Nein"} {
		if strings.Contains(content.Message, marker) {
			t.Errorf("Wahlgeschäft notice contains %q:\n%s", marker, content.Message)
		}
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// Each card's caption links that vote's own recording, at its segment, where the
// source has one: the post can only link the agenda item.
func TestFormatPost_CaptionsLinkTheVotesOwnRecording(t *testing.T) {
	group := testfixtures.KantonsratMultiVote()
	content, err := FormatPost(group)
	if err != nil {
		t.Fatalf("FormatPost: %v", err)
	}
	seen := map[string]bool{}
	for i, v := range group {
		caption := content.Captions[i+1]
		if v.VideoURL == "" {
			t.Fatalf("fixture vote %d has no archive link", i)
		}
		if !strings.Contains(caption, "🎬 Video: "+v.VideoURL) {
			t.Errorf("caption %d is missing the vote's own recording %s:\n%s", i+1, v.VideoURL, caption)
		}
		if seen[v.VideoURL] {
			t.Errorf("two votes share the recording link %s", v.VideoURL)
		}
		seen[v.VideoURL] = true
		// The business page is already in the post, so it is not repeated.
		if strings.Contains(caption, "Abstimmung: "+v.SourceURL) {
			t.Errorf("caption %d repeats the post's own Geschäft link", i+1)
		}
	}
	// The post itself links the agenda item, never one vote's segment.
	if strings.Contains(content.Message, "segmentUid") {
		t.Errorf("post text links a single vote's segment:\n%s", content.Message)
	}
}

// Stadt Zürich has no recording link, but each vote has its own page beside the
// Traktandum the post links.
func TestFormatPost_CaptionsLinkTheVotesOwnPage(t *testing.T) {
	group := testfixtures.MultiVoteGroup()
	content, err := FormatPost(group)
	if err != nil {
		t.Fatalf("FormatPost: %v", err)
	}
	for i, v := range group {
		if !strings.Contains(content.Captions[i+1], "🔗 Abstimmung: "+v.SourceURL) {
			t.Errorf("caption %d is missing the vote's own page %s:\n%s", i+1, v.SourceURL, content.Captions[i+1])
		}
	}
}
