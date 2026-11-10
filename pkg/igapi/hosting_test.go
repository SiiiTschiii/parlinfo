package igapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseRepo(t *testing.T) {
	// The valid case is what ${{ github.repository }} and GITHUB_REPOSITORY
	// both hand over verbatim. Every rejected case is something a hand-written
	// .env could hold, and each has to fail loudly: a half-parsed repo would
	// upload images to the wrong place, or to nowhere with a green run.
	owner, name, err := ParseRepo("SiiiTschiii/zuerichratsinfo")
	if err != nil {
		t.Fatalf("ParseRepo() unexpected error: %v", err)
	}
	if owner != "SiiiTschiii" || name != "zuerichratsinfo" {
		t.Errorf("ParseRepo() = %q, %q, want %q, %q", owner, name, "SiiiTschiii", "zuerichratsinfo")
	}

	for _, bad := range []string{"", "zuerichratsinfo", "/zuerichratsinfo", "SiiiTschiii/", "a/b/c", "/"} {
		if _, _, err := ParseRepo(bad); err == nil {
			t.Errorf("ParseRepo(%q) = nil error, want error", bad)
		}
	}
}

func TestImageHoster_PagesBaseURL(t *testing.T) {
	h := NewImageHoster("myorg", "myrepo", "token")
	expected := "https://myorg.github.io/myrepo"
	if got := h.PagesBaseURL(); got != expected {
		t.Errorf("PagesBaseURL() = %q, want %q", got, expected)
	}
}

func TestImageHoster_UploadImages(t *testing.T) {
	var requests []requestCapture

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, requestCapture{
			method: r.Method,
			path:   r.URL.Path,
			body:   string(body),
		})

		if r.Method == http.MethodGet {
			// getFileSHA: file not found (new file)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// PUT: create file
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"content":{"sha":"abc123"}}`))
	}))
	defer server.Close()

	h := NewImageHoster("testorg", "testrepo", "ghp_test")
	h.SetAPIBase(server.URL)

	images := [][]byte{
		{0xFF, 0xD8, 0xFF, 0xE0}, // fake JPEG
		{0xFF, 0xD8, 0xFF, 0xE1}, // another fake JPEG
	}
	names := []string{"img_0.jpg", "img_1.jpg"}

	urls, err := h.UploadImages(images, names)
	if err != nil {
		t.Fatalf("UploadImages: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}

	expectedURL0 := "https://testorg.github.io/testrepo/ig-images/img_0.jpg"
	if urls[0] != expectedURL0 {
		t.Errorf("urls[0] = %q, want %q", urls[0], expectedURL0)
	}

	// Verify PUT requests were made with correct content
	putCount := 0
	for _, req := range requests {
		if req.method == http.MethodPut {
			putCount++
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(req.body), &payload); err != nil {
				t.Fatalf("parse PUT body: %v", err)
			}
			if payload["branch"] != "gh-pages" {
				t.Errorf("expected branch=gh-pages, got %v", payload["branch"])
			}
			// Verify content is base64 encoded
			contentStr, ok := payload["content"].(string)
			if !ok {
				t.Error("expected content to be a string")
			}
			if _, err := base64.StdEncoding.DecodeString(contentStr); err != nil {
				t.Errorf("content is not valid base64: %v", err)
			}
		}
	}
	if putCount != 2 {
		t.Errorf("expected 2 PUT requests, got %d", putCount)
	}
}

func TestImageHoster_UploadImages_MismatchedLengths(t *testing.T) {
	h := NewImageHoster("org", "repo", "token")
	_, err := h.UploadImages([][]byte{{1}}, []string{"a.jpg", "b.jpg"})
	if err == nil {
		t.Fatal("expected error for mismatched lengths")
	}
}

func TestImageHoster_CleanupImages(t *testing.T) {
	var requests []requestCapture

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, requestCapture{
			method: r.Method,
			path:   r.URL.Path,
			body:   string(body),
		})

		if r.Method == http.MethodGet {
			// getFileSHA: file exists
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sha":"sha_to_delete"}`))
			return
		}

		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	h := NewImageHoster("testorg", "testrepo", "ghp_test")
	h.SetAPIBase(server.URL)

	err := h.CleanupImages([]string{"img_0.jpg", "img_1.jpg"})
	if err != nil {
		t.Fatalf("CleanupImages: %v", err)
	}

	// Verify DELETE requests include the correct SHA
	deleteCount := 0
	for _, req := range requests {
		if req.method == http.MethodDelete {
			deleteCount++
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(req.body), &payload); err != nil {
				t.Fatalf("parse DELETE body: %v", err)
			}
			if payload["sha"] != "sha_to_delete" {
				t.Errorf("expected sha=sha_to_delete, got %v", payload["sha"])
			}
			if payload["branch"] != "gh-pages" {
				t.Errorf("expected branch=gh-pages, got %v", payload["branch"])
			}
		}
	}
	if deleteCount != 2 {
		t.Errorf("expected 2 DELETE requests, got %d", deleteCount)
	}
}

func TestImageHoster_UploadImages_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer server.Close()

	h := NewImageHoster("org", "repo", "bad_token")
	h.SetAPIBase(server.URL)

	_, err := h.UploadImages([][]byte{{1}}, []string{"img.jpg"})
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error should mention 'forbidden': %v", err)
	}
}

func TestImageHoster_UploadExistingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// File exists — return SHA
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sha":"existing_sha"}`))
			return
		}
		if r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// Verify SHA is included for update
			if payload["sha"] != "existing_sha" {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"message":"sha mismatch"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"content":{"sha":"new_sha"}}`))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	h := NewImageHoster("org", "repo", "token")
	h.SetAPIBase(server.URL)

	urls, err := h.UploadImages([][]byte{{1}}, []string{"img.jpg"})
	if err != nil {
		t.Fatalf("UploadImages (existing file): %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
}

func TestImageHoster_SetHeaders(t *testing.T) {
	var capturedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	h := NewImageHoster("org", "repo", "ghp_mytoken")
	h.SetAPIBase(server.URL)

	// Trigger a request (getFileSHA will make a GET)
	_, _ = h.getFileSHA("test/path")

	if got := capturedHeaders.Get("Authorization"); got != "Bearer ghp_mytoken" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer ghp_mytoken")
	}
	if got := capturedHeaders.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
		t.Errorf("X-GitHub-Api-Version = %q, want %q", got, "2022-11-28")
	}
}

type requestCapture struct {
	method string
	path   string
	body   string
}

// conflictServer answers the first `conflicts` writes with 409, as the Contents
// API does when another writer moved gh-pages first, and every later one with
// success. Reads report an existing file whose SHA changes with each write
// attempt, so a test can tell whether a retry re-read it.
func conflictServer(t *testing.T, conflicts int, writes *[]requestCapture) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprintf(w, `{"sha":"sha-%d"}`, len(*writes))
			return
		}
		body, _ := io.ReadAll(r.Body)
		*writes = append(*writes, requestCapture{method: r.Method, path: r.URL.Path, body: string(body)})
		if len(*writes) <= conflicts {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message":"is at 1a2b but expected 3c4d"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// conflictHoster is a hoster against server that records its pauses instead of
// taking them.
func conflictHoster(server *httptest.Server, pauses *[]time.Duration) *ImageHoster {
	h := NewImageHoster("testorg", "testrepo", "ghp_test")
	h.SetAPIBase(server.URL)
	h.sleep = func(d time.Duration) { *pauses = append(*pauses, d) }
	return h
}

// Two channels' bots commit to gh-pages side by side, so a write can lose the
// race for the branch head. It must be tried again, with the file's SHA read
// afresh, rather than fail the post.
func TestImageHoster_UploadRetriesConflict(t *testing.T) {
	var writes []requestCapture
	var pauses []time.Duration
	h := conflictHoster(conflictServer(t, 2, &writes), &pauses)

	urls, err := h.UploadImages([][]byte{{0xFF, 0xD8}}, []string{"img_0.jpg"})
	if err != nil {
		t.Fatalf("UploadImages: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
	if len(writes) != 3 {
		t.Fatalf("expected 3 PUT attempts, got %d", len(writes))
	}
	for i, want := range []string{"sha-0", "sha-1", "sha-2"} {
		if !strings.Contains(writes[i].body, `"sha":"`+want+`"`) {
			t.Errorf("attempt %d sent %s, want sha %s", i+1, writes[i].body, want)
		}
	}
	if len(pauses) != 2 || pauses[0] != conflictBackoff || pauses[1] != 2*conflictBackoff {
		t.Errorf("pauses = %v, want [%v %v]", pauses, conflictBackoff, 2*conflictBackoff)
	}
}

func TestImageHoster_CleanupRetriesConflict(t *testing.T) {
	var writes []requestCapture
	var pauses []time.Duration
	h := conflictHoster(conflictServer(t, 1, &writes), &pauses)

	if err := h.CleanupImages([]string{"img_0.jpg"}); err != nil {
		t.Fatalf("CleanupImages: %v", err)
	}
	if len(writes) != 2 || writes[1].method != http.MethodDelete {
		t.Fatalf("expected 2 DELETE attempts, got %+v", writes)
	}
	if !strings.Contains(writes[1].body, `"sha":"sha-1"`) {
		t.Errorf("retry sent %s, want the re-read sha-1", writes[1].body)
	}
}

// A branch that never stops conflicting ends the attempts with an error that
// still names the conflict, and without a pause after the last one.
func TestImageHoster_UploadGivesUpOnLastingConflict(t *testing.T) {
	var writes []requestCapture
	var pauses []time.Duration
	h := conflictHoster(conflictServer(t, 1000, &writes), &pauses)

	_, err := h.UploadImages([][]byte{{0xFF, 0xD8}}, []string{"img_0.jpg"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, errConflict) || !strings.Contains(err.Error(), "409") {
		t.Errorf("error = %v, want one naming the 409 conflict", err)
	}
	if len(writes) != conflictAttempts {
		t.Errorf("expected %d PUT attempts, got %d", conflictAttempts, len(writes))
	}
	if len(pauses) != conflictAttempts-1 {
		t.Errorf("expected %d pauses, got %d", conflictAttempts-1, len(pauses))
	}
}

// Only a conflict is worth another attempt: any other refusal would be refused
// again.
func TestImageHoster_UploadDoesNotRetryOtherErrors(t *testing.T) {
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		puts++
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()
	var pauses []time.Duration
	h := conflictHoster(server, &pauses)

	if _, err := h.UploadImages([][]byte{{0xFF, 0xD8}}, []string{"img_0.jpg"}); err == nil {
		t.Fatal("expected an error")
	}
	if puts != 1 || len(pauses) != 0 {
		t.Errorf("expected 1 PUT and no pause, got %d PUTs and %d pauses", puts, len(pauses))
	}
}
