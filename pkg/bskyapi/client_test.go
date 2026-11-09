package bskyapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadBlob_SendsBytesAndReturnsBlob(t *testing.T) {
	blob := `{"$type":"blob","ref":{"$link":"bafkrei123"},"mimeType":"image/jpeg","size":3}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.repo.uploadBlob" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "image/jpeg" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer jwt" {
			t.Errorf("Authorization = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "abc" {
			t.Errorf("body = %q", body)
		}
		_, _ = w.Write([]byte(`{"blob":` + blob + `}`))
	}))
	defer srv.Close()

	got, err := UploadBlob(&Session{AccessJwt: "jwt", ServiceEndpoint: srv.URL}, []byte("abc"), "image/jpeg")
	if err != nil {
		t.Fatalf("UploadBlob: %v", err)
	}
	if string(got) != blob {
		t.Errorf("blob = %s, want %s", got, blob)
	}
}

func TestUploadBlob_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"BlobTooLarge"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	if _, err := UploadBlob(&Session{ServiceEndpoint: srv.URL}, []byte("abc"), "image/jpeg"); err == nil {
		t.Fatal("expected an error for a 400 response")
	}
}

// The blob goes back into the record exactly as the PDS returned it, with the
// alt text and the aspect ratio beside it.
func TestCreateRecord_EmbedsImages(t *testing.T) {
	var record map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Record map[string]interface{} `json:"record"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decoding payload: %v", err)
		}
		record = payload.Record
		_, _ = w.Write([]byte(`{"uri":"at://did:plc:x/app.bsky.feed.post/1","cid":"c"}`))
	}))
	defer srv.Close()

	blob := json.RawMessage(`{"$type":"blob","ref":{"$link":"bafkrei123"},"mimeType":"image/jpeg","size":3}`)
	_, err := CreateRecord(&Session{DID: "did:plc:x", ServiceEndpoint: srv.URL}, "", nil, nil,
		[]Image{{Blob: blob, Alt: "165 Ja, 0 Nein", Width: 1080, Height: 1350}})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}

	embed, ok := record["embed"].(map[string]interface{})
	if !ok {
		t.Fatalf("record has no embed: %v", record)
	}
	if embed["$type"] != "app.bsky.embed.images" {
		t.Errorf("embed $type = %v", embed["$type"])
	}
	img := embed["images"].([]interface{})[0].(map[string]interface{})
	if img["alt"] != "165 Ja, 0 Nein" {
		t.Errorf("alt = %v", img["alt"])
	}
	if ref := img["image"].(map[string]interface{})["ref"].(map[string]interface{}); ref["$link"] != "bafkrei123" {
		t.Errorf("blob ref = %v", ref)
	}
	ratio := img["aspectRatio"].(map[string]interface{})
	if ratio["width"] != float64(1080) || ratio["height"] != float64(1350) {
		t.Errorf("aspectRatio = %v", ratio)
	}
}

func TestCreateRecord_NoEmbedWithoutImages(t *testing.T) {
	var record map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Record map[string]interface{} `json:"record"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		record = payload.Record
		_, _ = w.Write([]byte(`{"uri":"at://did:plc:x/app.bsky.feed.post/1","cid":"c"}`))
	}))
	defer srv.Close()

	if _, err := CreateRecord(&Session{ServiceEndpoint: srv.URL}, "text", nil, nil, nil); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if _, ok := record["embed"]; ok {
		t.Errorf("text-only post carries an embed: %v", record["embed"])
	}
}

func TestCreateRecord_RejectsMoreThanFourImages(t *testing.T) {
	images := make([]Image, MaxImagesPerPost+1)
	if _, err := CreateRecord(&Session{}, "", nil, nil, images); err == nil {
		t.Fatal("expected an error for five images")
	}
}
