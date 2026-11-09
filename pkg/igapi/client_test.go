package igapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateMediaContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.FormValue("image_url") != "https://example.com/img.jpg" {
			t.Errorf("unexpected image_url: %s", r.FormValue("image_url"))
		}
		if r.FormValue("is_carousel_item") != "true" {
			t.Errorf("expected is_carousel_item=true, got %s", r.FormValue("is_carousel_item"))
		}
		if r.FormValue("access_token") != "tok123" {
			t.Errorf("unexpected access_token: %s", r.FormValue("access_token"))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"container_1"}`))
	}))
	defer server.Close()

	client := NewClient("ig_user_1", "tok123")
	client.SetAPIBase(server.URL)

	id, err := client.CreateMediaContainer("https://example.com/img.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "container_1" {
		t.Errorf("expected container_1, got %s", id)
	}
}

func TestCreateMediaContainer_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client := NewClient("ig_user_1", "tok123")
	client.SetAPIBase(server.URL)

	_, err := client.CreateMediaContainer("https://example.com/img.jpg")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestCreateCarouselContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.FormValue("media_type") != "CAROUSEL" {
			t.Errorf("expected media_type=CAROUSEL, got %s", r.FormValue("media_type"))
		}

		var children []string
		if err := json.Unmarshal([]byte(r.FormValue("children")), &children); err != nil {
			t.Fatalf("parsing children: %v", err)
		}
		if len(children) != 2 || children[0] != "c1" || children[1] != "c2" {
			t.Errorf("unexpected children: %v", children)
		}

		if r.FormValue("caption") != "test caption" {
			t.Errorf("unexpected caption: %s", r.FormValue("caption"))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"carousel_1"}`))
	}))
	defer server.Close()

	client := NewClient("ig_user_1", "tok123")
	client.SetAPIBase(server.URL)

	id, err := client.CreateCarouselContainer([]string{"c1", "c2"}, "test caption")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "carousel_1" {
		t.Errorf("expected carousel_1, got %s", id)
	}
}

func TestPublishContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.FormValue("creation_id") != "carousel_1" {
			t.Errorf("unexpected creation_id: %s", r.FormValue("creation_id"))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"media_1"}`))
	}))
	defer server.Close()

	client := NewClient("ig_user_1", "tok123")
	client.SetAPIBase(server.URL)

	id, err := client.PublishContainer("carousel_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "media_1" {
		t.Errorf("expected media_1, got %s", id)
	}
}

func TestPollContainerStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("fields") != "status_code" {
			t.Errorf("expected fields=status_code, got %s", r.URL.Query().Get("fields"))
		}
		if r.URL.Query().Has("access_token") {
			t.Error("access token sent in the query string")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
			t.Errorf("expected Authorization: Bearer tok123, got %q", got)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status_code":"FINISHED"}`))
	}))
	defer server.Close()

	client := NewClient("ig_user_1", "tok123")
	client.SetAPIBase(server.URL)

	status, err := client.PollContainerStatus("container_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "FINISHED" {
		t.Errorf("expected FINISHED, got %s", status)
	}
}

func TestPollContainerStatus_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`server error`))
	}))
	defer server.Close()

	client := NewClient("ig_user_1", "tok123")
	client.SetAPIBase(server.URL)

	_, err := client.PollContainerStatus("container_1")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestGetPermalink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/media_99" {
			t.Errorf("expected path /media_99, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("fields") != "permalink" {
			t.Errorf("expected fields=permalink, got %s", r.URL.Query().Get("fields"))
		}
		// The token must stay out of the URL, which transport errors echo
		if r.URL.Query().Has("access_token") {
			t.Error("access token sent in the query string")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
			t.Errorf("expected Authorization: Bearer tok123, got %q", got)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"permalink":"https://www.instagram.com/p/DAbC123/","id":"media_99"}`))
	}))
	defer server.Close()

	client := NewClient("ig_user_1", "tok123")
	client.SetAPIBase(server.URL)

	permalink, err := client.GetPermalink("media_99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if permalink != "https://www.instagram.com/p/DAbC123/" {
		t.Errorf("unexpected permalink %q", permalink)
	}
}

func TestGetPermalink_Errors(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
	}{
		"server error":    {http.StatusInternalServerError, `server error`},
		"empty permalink": {http.StatusOK, `{"id":"media_99"}`},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			client := NewClient("ig_user_1", "tok123")
			client.SetAPIBase(server.URL)

			if _, err := client.GetPermalink("media_99"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// TestRoundTrip_FullCarouselFlow tests the complete flow: create items → create carousel → publish
func TestRoundTrip_FullCarouselFlow(t *testing.T) {
	step := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		body, _ := io.ReadAll(r.Body)
		_ = body // consume body for POST requests with form encoding

		w.WriteHeader(http.StatusOK)
		switch {
		case step <= 3: // 3 CreateMediaContainer calls
			_, _ = fmt.Fprintf(w, `{"id":"child_%d"}`, step)
		case step == 4: // CreateCarouselContainer
			_, _ = w.Write([]byte(`{"id":"carousel_99"}`))
		case step == 5: // PublishContainer
			_, _ = w.Write([]byte(`{"id":"media_99"}`))
		}
	}))
	defer server.Close()

	client := NewClient("ig_user_1", "tok123")
	client.SetAPIBase(server.URL)

	// Step 1: Create child containers
	childIDs := make([]string, 3)
	for i := range childIDs {
		id, err := client.CreateMediaContainer("https://example.com/img.jpg")
		if err != nil {
			t.Fatalf("CreateMediaContainer %d: %v", i, err)
		}
		childIDs[i] = id
	}

	// Step 2: Create carousel container
	carouselID, err := client.CreateCarouselContainer(childIDs, "caption")
	if err != nil {
		t.Fatalf("CreateCarouselContainer: %v", err)
	}
	if carouselID != "carousel_99" {
		t.Errorf("expected carousel_99, got %s", carouselID)
	}

	// Step 3: Publish
	mediaID, err := client.PublishContainer(carouselID)
	if err != nil {
		t.Fatalf("PublishContainer: %v", err)
	}
	if mediaID != "media_99" {
		t.Errorf("expected media_99, got %s", mediaID)
	}
}

// TestGET_TransportErrorOmitsToken pins why the GET calls send the token as a
// header: a failed request's error names the URL, and postReal logs that error.
func TestGET_TransportErrorOmitsToken(t *testing.T) {
	client := NewClient("ig_user_1", "secret-token-123")
	client.SetAPIBase("http://127.0.0.1:1") // nothing listens here

	for name, call := range map[string]func(string) (string, error){
		"GetPermalink":        client.GetPermalink,
		"PollContainerStatus": client.PollContainerStatus,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := call("media_99")
			if err == nil {
				t.Fatal("expected a transport error")
			}
			if strings.Contains(err.Error(), "secret-token-123") {
				t.Errorf("error leaks the access token: %v", err)
			}
		})
	}
}
