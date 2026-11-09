package xapi

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeX points the client at a local server for the duration of a test and
// records each request's path and JSON body.
func fakeX(t *testing.T, respond func(path string) (int, string)) *[]recordedRequest {
	t.Helper()
	var reqs []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		if !strings.HasPrefix(r.Header.Get("Authorization"), "OAuth ") {
			t.Errorf("%s: request is not OAuth-signed", r.URL.Path)
		}
		reqs = append(reqs, recordedRequest{path: r.URL.Path, payload: payload})
		status, resp := respond(r.URL.Path)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(resp))
	}))
	old := apiBase
	apiBase = server.URL
	t.Cleanup(func() {
		apiBase = old
		server.Close()
	})
	return &reqs
}

type recordedRequest struct {
	path    string
	payload map[string]interface{}
}

func TestPostTweet_Payload(t *testing.T) {
	reqs := fakeX(t, func(string) (int, string) { return http.StatusCreated, `{"data":{"id":"111222333"}}` })

	if _, err := PostTweet("k", "s", "t", "ts", "hello", "", nil); err != nil {
		t.Fatalf("PostTweet: %v", err)
	}
	if _, err := PostTweet("k", "s", "t", "ts", "", "123456", []string{"9", "10"}); err != nil {
		t.Fatalf("PostTweet with media: %v", err)
	}

	plain := (*reqs)[0].payload
	if plain["text"] != "hello" {
		t.Errorf("text = %v", plain["text"])
	}
	for _, field := range []string{"reply", "media"} {
		if _, ok := plain[field]; ok {
			t.Errorf("plain tweet carries %q: %v", field, plain[field])
		}
	}

	withMedia := (*reqs)[1].payload
	if reply := withMedia["reply"].(map[string]interface{}); reply["in_reply_to_tweet_id"] != "123456" {
		t.Errorf("reply = %v", reply)
	}
	ids := withMedia["media"].(map[string]interface{})["media_ids"].([]interface{})
	if len(ids) != 2 || ids[0] != "9" || ids[1] != "10" {
		t.Errorf("media_ids = %v", ids)
	}
	if text, ok := withMedia["text"]; !ok || text != "" {
		t.Errorf("an image-only tweet should send empty text, got %v (present: %v)", text, ok)
	}
}

func TestPostTweet_RejectsMoreThanFourImages(t *testing.T) {
	reqs := fakeX(t, func(string) (int, string) { return http.StatusCreated, `{"data":{"id":"1"}}` })
	if _, err := PostTweet("k", "s", "t", "ts", "", "", []string{"1", "2", "3", "4", "5"}); err == nil {
		t.Fatal("expected an error for five media IDs")
	}
	if len(*reqs) != 0 {
		t.Errorf("a rejected tweet still reached the API")
	}
}

func TestUploadImage_SendsBase64AndReturnsID(t *testing.T) {
	reqs := fakeX(t, func(string) (int, string) {
		return http.StatusOK, `{"data":{"id":"1880028106020515840","media_key":"3_1880028106020515840"}}`
	})

	id, err := UploadImage("k", "s", "t", "ts", []byte{0xFF, 0xD8, 0xFF})
	if err != nil {
		t.Fatalf("UploadImage: %v", err)
	}
	if id != "1880028106020515840" {
		t.Errorf("id = %q", id)
	}
	req := (*reqs)[0]
	if req.path != "/2/media/upload" {
		t.Errorf("path = %s", req.path)
	}
	if req.payload["media"] != base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF}) {
		t.Errorf("media = %v", req.payload["media"])
	}
	if req.payload["media_category"] != "tweet_image" {
		t.Errorf("media_category = %v", req.payload["media_category"])
	}
}

func TestUploadImage_ErrorStatus(t *testing.T) {
	fakeX(t, func(string) (int, string) { return http.StatusForbidden, `{"title":"Forbidden"}` })
	if _, err := UploadImage("k", "s", "t", "ts", []byte{1}); err == nil {
		t.Fatal("expected an error for a 403")
	}
}

func TestSetAltText_PayloadAndCap(t *testing.T) {
	reqs := fakeX(t, func(string) (int, string) { return http.StatusOK, `{"data":{"id":"42"}}` })

	if err := SetAltText("k", "s", "t", "ts", "42", strings.Repeat("ä", MaxAltTextRunes+50)); err != nil {
		t.Fatalf("SetAltText: %v", err)
	}
	req := (*reqs)[0]
	if req.path != "/2/media/metadata" {
		t.Errorf("path = %s", req.path)
	}
	if req.payload["id"] != "42" {
		t.Errorf("id = %v", req.payload["id"])
	}
	text := req.payload["metadata"].(map[string]interface{})["alt_text"].(map[string]interface{})["text"].(string)
	if n := len([]rune(text)); n != MaxAltTextRunes {
		t.Errorf("alt text is %d runes, want it capped at %d", n, MaxAltTextRunes)
	}
}

func TestTweetResponse_ParsesID(t *testing.T) {
	body := []byte(`{"data":{"id":"1234567890"}}`)

	var resp tweetResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Data.ID != "1234567890" {
		t.Errorf("expected ID '1234567890', got %q", resp.Data.ID)
	}
}
