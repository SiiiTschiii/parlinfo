package xapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// tweetResponse represents the X API v2 response for tweet creation
type tweetResponse struct {
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

// apiBase is the X API host. A variable so tests can point the media calls at
// a local server.
var apiBase = "https://api.x.com"

// MaxMediaPerTweet is how many images one tweet can carry.
const MaxMediaPerTweet = 4

// MaxAltTextRunes is X's limit on an image description.
const MaxAltTextRunes = 1000

// PostTweet posts a tweet using X API v2 with OAuth 1.0a User Context.
// If inReplyToTweetID is non-empty, the tweet is posted as a reply.
// mediaIDs are optional, at most MaxMediaPerTweet, each from UploadImage.
// Returns the created tweet's ID.
func PostTweet(apiKey, apiSecret, accessToken, accessSecret, message, inReplyToTweetID string, mediaIDs []string) (string, error) {
	// X API v2 endpoint for creating tweets
	apiURL := apiBase + "/2/tweets"

	if len(mediaIDs) > MaxMediaPerTweet {
		return "", fmt.Errorf("a tweet holds at most %d images, got %d", MaxMediaPerTweet, len(mediaIDs))
	}

	// Create the tweet payload
	payload := tweetPayload(message, inReplyToTweetID, mediaIDs)
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tweet payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Generate OAuth 1.0a authorization header
	authHeader := generateOAuthHeader(apiURL, "POST", apiKey, apiSecret, accessToken, accessSecret)
	req.Header.Set("Authorization", authHeader)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to post tweet: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("x API returned status %d: %s", resp.StatusCode, string(body))
	}

	var tweetResp tweetResponse
	if err := json.Unmarshal(body, &tweetResp); err != nil {
		return "", fmt.Errorf("failed to parse tweet response: %w", err)
	}

	fmt.Printf("✅ Tweet posted successfully! (ID: %s)\n", tweetResp.Data.ID)
	return tweetResp.Data.ID, nil
}

// tweetPayload builds the POST /2/tweets body. The text is sent even when
// empty: X accepts a tweet with media and no text.
func tweetPayload(message, inReplyToTweetID string, mediaIDs []string) map[string]interface{} {
	payload := map[string]interface{}{
		"text": message,
	}
	if inReplyToTweetID != "" {
		payload["reply"] = map[string]interface{}{
			"in_reply_to_tweet_id": inReplyToTweetID,
		}
	}
	if len(mediaIDs) > 0 {
		payload["media"] = map[string]interface{}{
			"media_ids": mediaIDs,
		}
	}
	return payload
}

// UploadImage uploads a JPEG through the one-shot v2 media endpoint and
// returns its media ID, ready for PostTweet.
//
// The image goes base64-encoded in a JSON body rather than as multipart: OAuth
// 1.0a signs neither kind of body, and JSON spares building a multipart one.
// An uploaded image no tweet uses expires on its own, so a failed post leaves
// nothing behind to clean up.
func UploadImage(apiKey, apiSecret, accessToken, accessSecret string, jpeg []byte) (string, error) {
	body, err := postJSON(apiBase+"/2/media/upload", apiKey, apiSecret, accessToken, accessSecret, map[string]string{
		"media":          base64.StdEncoding.EncodeToString(jpeg),
		"media_category": "tweet_image",
	})
	if err != nil {
		return "", fmt.Errorf("uploading image: %w", err)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("failed to parse media upload response: %w", err)
	}
	if out.Data.ID == "" {
		return "", fmt.Errorf("media upload response has no id: %s", string(body))
	}
	return out.Data.ID, nil
}

// SetAltText attaches an image description to uploaded media. It has to run
// before the tweet that uses the media is posted.
func SetAltText(apiKey, apiSecret, accessToken, accessSecret, mediaID, alt string) error {
	if runes := []rune(alt); len(runes) > MaxAltTextRunes {
		alt = string(runes[:MaxAltTextRunes-1]) + "…"
	}
	_, err := postJSON(apiBase+"/2/media/metadata", apiKey, apiSecret, accessToken, accessSecret, map[string]interface{}{
		"id": mediaID,
		"metadata": map[string]interface{}{
			"alt_text": map[string]string{"text": alt},
		},
	})
	if err != nil {
		return fmt.Errorf("setting alt text: %w", err)
	}
	return nil
}

// postJSON sends an OAuth 1.0a-signed JSON POST and returns the response body
// of a 2xx response.
func postJSON(apiURL, apiKey, apiSecret, accessToken, accessSecret string, payload interface{}) ([]byte, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", generateOAuthHeader(apiURL, "POST", apiKey, apiSecret, accessToken, accessSecret))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("x API returned status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// GetAuthenticatedUserID returns the user ID of the authenticated user.
func GetAuthenticatedUserID(apiKey, apiSecret, accessToken, accessSecret string) (string, error) {
	apiURL := "https://api.x.com/2/users/me"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	authHeader := generateOAuthHeader(apiURL, "GET", apiKey, apiSecret, accessToken, accessSecret)
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("x API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse user response: %w", err)
	}
	return result.Data.ID, nil
}

// GetUserTweets returns tweet IDs for a given user (most recent first, up to 100).
func GetUserTweets(apiKey, apiSecret, accessToken, accessSecret, userID string) ([]string, error) {
	apiURL := "https://api.x.com/2/users/" + userID + "/tweets"
	fullURL := apiURL + "?max_results=100"

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// OAuth signature must be computed on base URL; query params added separately
	authHeader := generateOAuthHeaderWithParams(apiURL, "GET", apiKey, apiSecret, accessToken, accessSecret, map[string]string{"max_results": "100"})
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get tweets: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("x API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tweets response: %w", err)
	}

	ids := make([]string, len(result.Data))
	for i, t := range result.Data {
		ids[i] = t.ID
	}
	return ids, nil
}

// DeleteTweet deletes a tweet by ID.
func DeleteTweet(apiKey, apiSecret, accessToken, accessSecret, tweetID string) error {
	apiURL := "https://api.x.com/2/tweets/" + tweetID

	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	authHeader := generateOAuthHeader(apiURL, "DELETE", apiKey, apiSecret, accessToken, accessSecret)
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete tweet: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("x API returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// generateOAuthHeader generates the OAuth 1.0a authorization header
func generateOAuthHeader(apiURL, method, apiKey, apiSecret, accessToken, accessSecret string) string {
	return generateOAuthHeaderWithParams(apiURL, method, apiKey, apiSecret, accessToken, accessSecret, nil)
}

// generateOAuthHeaderWithParams generates the OAuth 1.0a authorization header,
// including additional query parameters in the signature base string.
func generateOAuthHeaderWithParams(apiURL, method, apiKey, apiSecret, accessToken, accessSecret string, queryParams map[string]string) string {
	// OAuth parameters
	params := map[string]string{
		"oauth_consumer_key":     apiKey,
		"oauth_nonce":            generateNonce(),
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        strconv.FormatInt(time.Now().Unix(), 10),
		"oauth_token":            accessToken,
		"oauth_version":          "1.0",
	}

	// Include query params in signature computation
	allParams := make(map[string]string)
	for k, v := range params {
		allParams[k] = v
	}
	for k, v := range queryParams {
		allParams[k] = v
	}

	// Generate signature using all params
	signature := generateSignature(method, apiURL, allParams, apiSecret, accessSecret)
	params["oauth_signature"] = signature

	// Build authorization header (only OAuth params, not query params)
	var authParts []string
	for key, value := range params {
		authParts = append(authParts, fmt.Sprintf(`%s="%s"`, key, percentEncode(value)))
	}

	// Sort for consistency
	sort.Strings(authParts)

	return "OAuth " + strings.Join(authParts, ", ")
}

// generateSignature generates the OAuth signature
func generateSignature(method, apiURL string, params map[string]string, apiSecret, accessSecret string) string {
	// Create parameter string
	var paramPairs []string
	for key, value := range params {
		paramPairs = append(paramPairs, percentEncode(key)+"="+percentEncode(value))
	}

	// Sort parameters
	sort.Strings(paramPairs)
	paramString := strings.Join(paramPairs, "&")

	// Create signature base string
	baseString := method + "&" + percentEncode(apiURL) + "&" + percentEncode(paramString)

	// Create signing key
	signingKey := percentEncode(apiSecret) + "&" + percentEncode(accessSecret)

	// Generate HMAC-SHA1 signature
	mac := hmac.New(sha1.New, []byte(signingKey))
	mac.Write([]byte(baseString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature
}

// generateNonce generates a random nonce for OAuth
func generateNonce() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

// percentEncode encodes a string per RFC 3986
func percentEncode(s string) string {
	return url.QueryEscape(s)
}
