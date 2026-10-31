// Package testcontacts gives the Geschäft formatter tests a contacts.Mapper
// with known handles, so tagging is exercised without the real contact list.
package testcontacts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
)

// Handles every platform tags Brigitte Fürer with — the first signer of
// testfixtures.MotionMember.
const (
	XHandle         = "BrigitteFuerer"
	BlueskyHandle   = "brigittefuerer.bsky.social"
	InstagramHandle = "brigitte.fuerer"
)

const yaml = `version: "1.0"
contacts:
  - name: Brigitte Fürer
    bluesky:
      - url: https://bsky.app/profile/` + BlueskyHandle + `
        verified: true
    instagram:
      - url: https://www.instagram.com/` + InstagramHandle + `/
        verified: true
    x:
      - url: https://x.com/` + XHandle + `
        verified: true
`

// Mapper writes the contacts to a temp file and loads them.
func Mapper(t *testing.T) *contacts.Mapper {
	t.Helper()
	path := filepath.Join(t.TempDir(), "contacts.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := contacts.LoadContacts(path)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
