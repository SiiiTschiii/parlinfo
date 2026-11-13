package igapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultGraphAPIBase is the base URL for the Instagram Graph API.
	DefaultGraphAPIBase = "https://graph.facebook.com/v25.0"

	// StatusPublished indicates the container has been published successfully.
	StatusPublished = "PUBLISHED"
	// StatusFinished indicates the container is ready to be published.
	StatusFinished = "FINISHED"
	// StatusInProgress indicates the container is still being processed.
	StatusInProgress = "IN_PROGRESS"
	// StatusError indicates the container encountered an error.
	StatusError = "ERROR"
	// StatusExpired indicates the container has expired.
	StatusExpired = "EXPIRED"
)

// Client is an Instagram Graph API client for carousel publishing.
type Client struct {
	igUserID    string
	accessToken string
	httpClient  *http.Client
	apiBase     string // base URL for the Graph API (overridable for testing)
}

// NewClient creates a new Instagram API client.
func NewClient(igUserID, accessToken string) *Client {
	return &Client{
		igUserID:    igUserID,
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		apiBase:     DefaultGraphAPIBase,
	}
}

// SetAPIBase overrides the Graph API base URL (for testing).
func (c *Client) SetAPIBase(base string) {
	c.apiBase = base
}

// idResponse is the common response shape for IG API calls that return an ID.
type idResponse struct {
	ID string `json:"id"`
}

// statusResponse is the response from querying a container's status.
type statusResponse struct {
	StatusCode string `json:"status_code"`
}

// Media is one of the account's published media objects.
type Media struct {
	ID        string `json:"id"`
	Caption   string `json:"caption"`
	Permalink string `json:"permalink"`
	// Children lists a carousel's items; it is empty for a single image.
	Children struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	} `json:"children"`
}

// ImageCount is how many images the post shows: a carousel's items, or one.
func (m Media) ImageCount() int {
	if n := len(m.Children.Data); n > 0 {
		return n
	}
	return 1
}

// mediaListResponse is the response from listing the account's media.
type mediaListResponse struct {
	Data []Media `json:"data"`
}

// permalinkResponse is the response from querying a media object's permalink.
type permalinkResponse struct {
	Permalink string `json:"permalink"`
}

// CreateMediaContainer creates a carousel item container for a single image.
// The imageURL must be a publicly accessible JPEG URL.
// Returns the container ID.
func (c *Client) CreateMediaContainer(imageURL string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/media", c.apiBase, c.igUserID)

	params := url.Values{}
	params.Set("image_url", imageURL)
	params.Set("is_carousel_item", "true")
	params.Set("access_token", c.accessToken)

	resp, err := c.httpClient.PostForm(endpoint, params)
	if err != nil {
		return "", fmt.Errorf("creating media container: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create media container returned status %d: %s", resp.StatusCode, string(body))
	}

	var result idResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing media container response: %w", err)
	}

	return result.ID, nil
}

// CreateSingleImageContainer creates a standalone image container with a caption.
// Use this instead of a carousel when there is only one image (carousels require 2+ images).
// Returns the container ID.
func (c *Client) CreateSingleImageContainer(imageURL, caption string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/media", c.apiBase, c.igUserID)

	params := url.Values{}
	params.Set("image_url", imageURL)
	params.Set("caption", caption)
	params.Set("access_token", c.accessToken)

	resp, err := c.httpClient.PostForm(endpoint, params)
	if err != nil {
		return "", fmt.Errorf("creating single image container: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create single image container returned status %d: %s", resp.StatusCode, string(body))
	}

	var result idResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing single image container response: %w", err)
	}

	return result.ID, nil
}

// CreateCarouselContainer creates a carousel container with child containers and a caption.
// Returns the carousel container ID.
func (c *Client) CreateCarouselContainer(childIDs []string, caption string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/media", c.apiBase, c.igUserID)

	// Build children JSON array
	childrenJSON, err := json.Marshal(childIDs)
	if err != nil {
		return "", fmt.Errorf("marshaling child IDs: %w", err)
	}

	params := url.Values{}
	params.Set("media_type", "CAROUSEL")
	params.Set("children", string(childrenJSON))
	params.Set("caption", caption)
	params.Set("access_token", c.accessToken)

	resp, err := c.httpClient.PostForm(endpoint, params)
	if err != nil {
		return "", fmt.Errorf("creating carousel container: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create carousel container returned status %d: %s", resp.StatusCode, string(body))
	}

	var result idResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing carousel container response: %w", err)
	}

	return result.ID, nil
}

// PublishContainer publishes a previously created container (carousel or single image).
// Returns the published media ID.
func (c *Client) PublishContainer(containerID string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/media_publish", c.apiBase, c.igUserID)

	params := url.Values{}
	params.Set("creation_id", containerID)
	params.Set("access_token", c.accessToken)

	resp, err := c.httpClient.PostForm(endpoint, params)
	if err != nil {
		return "", fmt.Errorf("publishing container: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("publish container returned status %d: %s", resp.StatusCode, string(body))
	}

	var result idResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing publish response: %w", err)
	}

	return result.ID, nil
}

// PollContainerStatus checks the status of a container.
// Returns one of: PUBLISHED, FINISHED, IN_PROGRESS, ERROR, EXPIRED.
//
// Like GetPermalink, it sends the token as a header to keep it out of the URL
// that a transport error carries into the log.
func (c *Client) PollContainerStatus(containerID string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s", c.apiBase, containerID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("creating status request: %w", err)
	}

	q := req.URL.Query()
	q.Set("fields", "status_code")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("polling container status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("poll container status returned %d: %s", resp.StatusCode, string(body))
	}

	var result statusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing status response: %w", err)
	}

	return result.StatusCode, nil
}

// GetPermalink returns the public URL of a published media object, from the ID
// PublishContainer returned. Instagram's /p/{shortcode} URLs cannot be derived
// from the media ID, so this costs one call; instagram_basic covers it.
//
// The token travels in the Authorization header rather than the query string:
// a transport failure comes back as a *url.Error carrying the request URL, and
// the caller logs it, so a token in the URL would land in the log.
func (c *Client) GetPermalink(mediaID string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s", c.apiBase, mediaID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("creating permalink request: %w", err)
	}

	q := req.URL.Query()
	q.Set("fields", "permalink")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching permalink: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch permalink returned %d: %s", resp.StatusCode, string(body))
	}

	var result permalinkResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing permalink response: %w", err)
	}
	if result.Permalink == "" {
		return "", fmt.Errorf("permalink response carried no permalink: %s", string(body))
	}

	return result.Permalink, nil
}

// RecentMedia returns the account's latest published media, newest first, at
// most limit of them. instagram_basic covers it, as it does GetPermalink, and
// the token travels in the header for the same reason.
func (c *Client) RecentMedia(limit int) ([]Media, error) {
	endpoint := fmt.Sprintf("%s/%s/media", c.apiBase, c.igUserID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating media list request: %w", err)
	}

	q := req.URL.Query()
	q.Set("fields", "id,caption,permalink,children{id}")
	q.Set("limit", fmt.Sprint(limit))
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing media: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list media returned %d: %s", resp.StatusCode, string(body))
	}

	var result mediaListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing media list response: %w", err)
	}

	return result.Data, nil
}
