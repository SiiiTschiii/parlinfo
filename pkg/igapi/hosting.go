package igapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	// defaultGitHubAPIBase is the base URL for the GitHub REST API.
	defaultGitHubAPIBase = "https://api.github.com"

	// ghPagesImageDir is the directory on the gh-pages branch where images are hosted.
	ghPagesImageDir = "ig-images"

	// conflictAttempts is how often one write to gh-pages is tried before a
	// conflict is given up on, and conflictBackoff the pause after the first
	// conflict; it doubles after each further one (0.5s, 1s, 2s, 4s).
	conflictAttempts = 5
	conflictBackoff  = 500 * time.Millisecond
)

// errConflict marks a write the Contents API refused with 409: the branch
// moved between GitHub reading its head and committing onto it, because
// another writer — another channel's bot — committed in between.
var errConflict = errors.New("conflict")

// ImageHoster uploads and cleans up images on GitHub Pages for Instagram to fetch.
type ImageHoster struct {
	repoOwner   string
	repoName    string
	githubToken string
	httpClient  *http.Client
	apiBase     string              // overridable for testing
	sleep       func(time.Duration) // overridable for testing
}

// ParseRepo splits an "owner/name" repository reference, the shape GitHub uses
// everywhere including GITHUB_REPOSITORY. Configuration carries the repo as one
// value so it can be passed through from ${{ github.repository }} unmodified;
// this is where it becomes the two fields the API needs.
func ParseRepo(repo string) (owner, name string, err error) {
	owner, name, found := strings.Cut(repo, "/")
	if !found || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("malformed repository %q: want \"owner/name\"", repo)
	}
	return owner, name, nil
}

// NewImageHoster creates a new GitHub Pages image hoster.
func NewImageHoster(repoOwner, repoName, githubToken string) *ImageHoster {
	return &ImageHoster{
		repoOwner:   repoOwner,
		repoName:    repoName,
		githubToken: githubToken,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		apiBase:     defaultGitHubAPIBase,
		sleep:       time.Sleep,
	}
}

// SetAPIBase overrides the GitHub API base URL (for testing).
func (h *ImageHoster) SetAPIBase(base string) {
	h.apiBase = base
}

// PagesBaseURL returns the GitHub Pages base URL for this repository.
// GitHub Pages domains are always lowercase.
func (h *ImageHoster) PagesBaseURL() string {
	return fmt.Sprintf("https://%s.github.io/%s", strings.ToLower(h.repoOwner), h.repoName)
}

// UploadImages commits JPEG images to the gh-pages branch and returns their public URLs.
// Each image is uploaded as a separate file under the ig-images/ directory.
// names[i] corresponds to images[i].
func (h *ImageHoster) UploadImages(images [][]byte, names []string) ([]string, error) {
	if len(images) != len(names) {
		return nil, fmt.Errorf("images and names length mismatch: %d vs %d", len(images), len(names))
	}

	urls := make([]string, len(images))
	for i, imgData := range images {
		path := ghPagesImageDir + "/" + names[i]
		if err := h.putFileContent(path, imgData, fmt.Sprintf("upload %s for Instagram carousel", names[i])); err != nil {
			return nil, fmt.Errorf("uploading %s: %w", names[i], err)
		}
		urls[i] = fmt.Sprintf("%s/%s", h.PagesBaseURL(), path)
	}

	return urls, nil
}

// CleanupImages removes previously uploaded images from the gh-pages branch.
func (h *ImageHoster) CleanupImages(names []string) error {
	for _, name := range names {
		path := ghPagesImageDir + "/" + name
		if err := h.deleteFileContent(path, fmt.Sprintf("cleanup %s after Instagram publish", name)); err != nil {
			return fmt.Errorf("deleting %s: %w", name, err)
		}
	}
	return nil
}

// retryOnConflict runs one write to gh-pages, again after a pause for as long
// as it reports errConflict. Every channel's bot commits to that one branch and
// they run side by side, so a conflict is expected now and then; without the
// retry it would fail the post, or leave an image behind after cleanup.
func (h *ImageHoster) retryOnConflict(write func() error) error {
	backoff := conflictBackoff
	var err error
	for attempt := 1; attempt <= conflictAttempts; attempt++ {
		if err = write(); !errors.Is(err, errConflict) {
			return err
		}
		if attempt < conflictAttempts {
			log.Printf("⚠️  gh-pages conflict (attempt %d of %d), retrying in %v", attempt, conflictAttempts, backoff)
			h.sleep(backoff)
			backoff *= 2
		}
	}
	return fmt.Errorf("still conflicting after %d attempts: %w", conflictAttempts, err)
}

// putFileContent creates or updates a file on the gh-pages branch, retrying
// when another writer got in first.
func (h *ImageHoster) putFileContent(path string, content []byte, message string) error {
	return h.retryOnConflict(func() error { return h.putFileContentOnce(path, content, message) })
}

// putFileContentOnce makes a single attempt using the GitHub Contents API.
// PUT /repos/{owner}/{repo}/contents/{path}
func (h *ImageHoster) putFileContentOnce(path string, content []byte, message string) error {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/contents/%s", h.apiBase, h.repoOwner, h.repoName, path)

	// Check if file already exists to get its SHA (needed for update)
	sha, _ := h.getFileSHA(path)

	payload := map[string]interface{}{
		"message": message,
		"content": encodeBase64(content),
		"branch":  "gh-pages",
	}
	if sha != "" {
		payload["sha"] = sha
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequest("PUT", endpoint, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	h.setHeaders(req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("PUT %s returned status %d: %s: %w", path, resp.StatusCode, string(body), errConflict)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("PUT %s returned status %d: %s", path, resp.StatusCode, string(body))
	}

	return nil
}

// deleteFileContent removes a file from the gh-pages branch, retrying when
// another writer got in first.
func (h *ImageHoster) deleteFileContent(path, message string) error {
	return h.retryOnConflict(func() error { return h.deleteFileContentOnce(path, message) })
}

// deleteFileContentOnce makes a single attempt using the GitHub Contents API.
// DELETE /repos/{owner}/{repo}/contents/{path}
func (h *ImageHoster) deleteFileContentOnce(path, message string) error {
	sha, err := h.getFileSHA(path)
	if err != nil {
		return fmt.Errorf("getting SHA for %s: %w", path, err)
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/contents/%s", h.apiBase, h.repoOwner, h.repoName, path)

	payload := map[string]interface{}{
		"message": message,
		"sha":     sha,
		"branch":  "gh-pages",
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequest("DELETE", endpoint, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	h.setHeaders(req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("DELETE %s returned status %d: %s: %w", path, resp.StatusCode, string(body), errConflict)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DELETE %s returned status %d: %s", path, resp.StatusCode, string(body))
	}

	return nil
}

// getFileSHA returns the SHA of a file on the gh-pages branch, or empty string if not found.
func (h *ImageHoster) getFileSHA(path string) (string, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=gh-pages", h.apiBase, h.repoOwner, h.repoName, path)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	h.setHeaders(req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s returned status %d: %s", path, resp.StatusCode, string(body))
	}

	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing file response: %w", err)
	}
	return result.SHA, nil
}

// setHeaders sets common headers for GitHub API requests.
func (h *ImageHoster) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+h.githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

// encodeBase64 encodes bytes to base64 string (standard encoding).
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
