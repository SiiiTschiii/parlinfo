package bskyapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultPDSHost = "https://bsky.social"

// Session holds the authenticated session data
type Session struct {
	DID             string `json:"did"`
	Handle          string `json:"handle"`
	AccessJwt       string `json:"accessJwt"`
	RefreshJwt      string `json:"refreshJwt"`
	ServiceEndpoint string // resolved from didDoc
}

// sessionResponse is the raw API response from createSession
type sessionResponse struct {
	DID        string `json:"did"`
	Handle     string `json:"handle"`
	AccessJwt  string `json:"accessJwt"`
	RefreshJwt string `json:"refreshJwt"`
	DidDoc     struct {
		Service []struct {
			ID              string `json:"id"`
			Type            string `json:"type"`
			ServiceEndpoint string `json:"serviceEndpoint"`
		} `json:"service"`
	} `json:"didDoc"`
}

// Facet represents a rich text annotation (link, mention, etc.)
type Facet struct {
	Index    FacetIndex     `json:"index"`
	Features []FacetFeature `json:"features"`
}

// FacetIndex represents the byte range of a facet in the text
type FacetIndex struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

// FacetFeature represents a facet feature (link, mention, tag)
type FacetFeature struct {
	Type string `json:"$type"`
	URI  string `json:"uri,omitempty"`
	DID  string `json:"did,omitempty"`
	Tag  string `json:"tag,omitempty"`
}

// CreateSession authenticates with the Bluesky AT Protocol and returns a session
func CreateSession(handle, password string) (*Session, error) {
	url := defaultPDSHost + "/xrpc/com.atproto.server.createSession"

	payload := map[string]string{
		"identifier": handle,
		"password":   password,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create session request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bluesky createSession returned status %d: %s", resp.StatusCode, string(body))
	}

	var sessResp sessionResponse
	if err := json.Unmarshal(body, &sessResp); err != nil {
		return nil, fmt.Errorf("failed to parse session response: %w", err)
	}

	// Resolve the PDS service endpoint from the didDoc
	serviceEndpoint := defaultPDSHost
	for _, svc := range sessResp.DidDoc.Service {
		if svc.Type == "AtprotoPersonalDataServer" {
			serviceEndpoint = svc.ServiceEndpoint
			break
		}
	}

	return &Session{
		DID:             sessResp.DID,
		Handle:          sessResp.Handle,
		AccessJwt:       sessResp.AccessJwt,
		RefreshJwt:      sessResp.RefreshJwt,
		ServiceEndpoint: serviceEndpoint,
	}, nil
}

// PostRef is a strong reference to a post (URI + CID), returned by CreateRecord
type PostRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

// ReplyRef contains the root and parent references for a reply post
type ReplyRef struct {
	Root   PostRef `json:"root"`
	Parent PostRef `json:"parent"`
}

// MaxImagesPerPost is how many images one app.bsky.embed.images embed holds.
const MaxImagesPerPost = 4

// Image is an uploaded image to embed in a post. Blob is the blob object
// UploadBlob returned, passed back untouched; Width and Height give the
// aspect ratio, without which clients crop the image to a default box.
type Image struct {
	Blob   json.RawMessage
	Alt    string
	Width  int
	Height int
}

// UploadBlob uploads data to the session's PDS and returns the blob object a
// post embeds to reference it.
//
// An uploaded blob that no record references is garbage-collected by the PDS,
// so a failed post leaves nothing behind to clean up.
func UploadBlob(session *Session, data []byte, mimeType string) (json.RawMessage, error) {
	url := session.ServiceEndpoint + "/xrpc/com.atproto.repo.uploadBlob"

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload blob: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bluesky uploadBlob returned status %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Blob json.RawMessage `json:"blob"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse uploadBlob response: %w", err)
	}
	if len(out.Blob) == 0 {
		return nil, fmt.Errorf("bluesky uploadBlob response has no blob: %s", string(body))
	}
	return out.Blob, nil
}

// imagesEmbed builds the app.bsky.embed.images object for a post.
func imagesEmbed(images []Image) map[string]interface{} {
	items := make([]map[string]interface{}, len(images))
	for i, img := range images {
		item := map[string]interface{}{
			"image": img.Blob,
			"alt":   img.Alt,
		}
		if img.Width > 0 && img.Height > 0 {
			item["aspectRatio"] = map[string]int{"width": img.Width, "height": img.Height}
		}
		items[i] = item
	}
	return map[string]interface{}{
		"$type":  "app.bsky.embed.images",
		"images": items,
	}
}

// CreateRecord creates a post on Bluesky and returns the post reference.
// text is the post content, facets are optional rich text annotations (links, mentions).
// replyTo is optional — if non-nil, the post is created as a reply in a thread.
// images are optional, at most MaxImagesPerPost, each uploaded with UploadBlob first.
func CreateRecord(session *Session, text string, facets []Facet, replyTo *ReplyRef, images []Image) (*PostRef, error) {
	if len(images) > MaxImagesPerPost {
		return nil, fmt.Errorf("a post holds at most %d images, got %d", MaxImagesPerPost, len(images))
	}

	url := session.ServiceEndpoint + "/xrpc/com.atproto.repo.createRecord"

	// Build the record
	record := map[string]interface{}{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	if len(facets) > 0 {
		record["facets"] = facets
	}

	if len(images) > 0 {
		record["embed"] = imagesEmbed(images)
	}

	if replyTo != nil {
		record["reply"] = map[string]interface{}{
			"root": map[string]string{
				"uri": replyTo.Root.URI,
				"cid": replyTo.Root.CID,
			},
			"parent": map[string]string{
				"uri": replyTo.Parent.URI,
				"cid": replyTo.Parent.CID,
			},
		}
	}

	payload := map[string]interface{}{
		"repo":       session.DID,
		"collection": "app.bsky.feed.post",
		"record":     record,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal post payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create post request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create record: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bluesky createRecord returned status %d: %s", resp.StatusCode, string(body))
	}

	var ref PostRef
	if err := json.Unmarshal(body, &ref); err != nil {
		return nil, fmt.Errorf("failed to parse createRecord response: %w", err)
	}

	return &ref, nil
}

// LinkFacet creates a facet for a URL link in the post text.
// byteStart and byteEnd are the byte offsets of the link text in the UTF-8 encoded post.
func LinkFacet(byteStart, byteEnd int, uri string) Facet {
	return Facet{
		Index: FacetIndex{
			ByteStart: byteStart,
			ByteEnd:   byteEnd,
		},
		Features: []FacetFeature{
			{
				Type: "app.bsky.richtext.facet#link",
				URI:  uri,
			},
		},
	}
}

// MentionFacet creates a facet for a mention in the post text.
// byteStart and byteEnd are the byte offsets of the mention text in the UTF-8 encoded post.
// did is the DID of the mentioned user (e.g. "did:plc:p3wcrhc5fj3hfkhoujdsyasy").
func MentionFacet(byteStart, byteEnd int, did string) Facet {
	return Facet{
		Index: FacetIndex{
			ByteStart: byteStart,
			ByteEnd:   byteEnd,
		},
		Features: []FacetFeature{
			{
				Type: "app.bsky.richtext.facet#mention",
				DID:  did,
			},
		},
	}
}

// ResolveHandle resolves a Bluesky handle to a DID using the public API.
// No authentication required.
func ResolveHandle(handle string) (string, error) {
	url := defaultPDSHost + "/xrpc/com.atproto.identity.resolveHandle?handle=" + handle

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to resolve handle %q: %w", handle, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolveHandle for %q returned status %d: %s", handle, resp.StatusCode, string(body))
	}

	var result struct {
		DID string `json:"did"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse resolveHandle response: %w", err)
	}

	return result.DID, nil
}

// ListRecords returns record URIs for the given collection (e.g. "app.bsky.feed.post")
// from the authenticated user's repo. Returns up to 100 records.
func ListRecords(session *Session, collection string) ([]string, error) {
	reqURL := session.ServiceEndpoint + "/xrpc/com.atproto.repo.listRecords?repo=" +
		url.QueryEscape(session.DID) + "&collection=" + url.QueryEscape(collection) + "&limit=100"

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create listRecords request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bluesky listRecords returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Records []struct {
			URI string `json:"uri"`
		} `json:"records"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse listRecords response: %w", err)
	}

	uris := make([]string, len(result.Records))
	for i, r := range result.Records {
		uris[i] = r.URI
	}
	return uris, nil
}

// DeleteRecord deletes a record by its AT URI.
// The URI format is: at://did:plc:.../collection/rkey
func DeleteRecord(session *Session, uri string) error {
	// Parse rkey from URI: at://did/collection/rkey
	parts := strings.SplitN(uri, "/", 5)
	if len(parts) < 5 {
		return fmt.Errorf("invalid AT URI: %s", uri)
	}
	collection := parts[3]
	rkey := parts[4]

	reqURL := session.ServiceEndpoint + "/xrpc/com.atproto.repo.deleteRecord"

	payload := map[string]string{
		"repo":       session.DID,
		"collection": collection,
		"rkey":       rkey,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal deleteRecord payload: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create deleteRecord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bluesky deleteRecord returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
