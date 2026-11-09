package main

import (
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/config"
)

func TestFacebookPostFrom(t *testing.T) {
	zurich := config.Channel{Key: "zurich"}
	day := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	dated := config.Jurisdiction{PostFrom: day}
	dated.Key = "nationalrat"
	undated := config.Jurisdiction{}
	undated.Key = "zurich-canton"

	t.Run("its own date wins and needs no jurisdiction date", func(t *testing.T) {
		t.Setenv("ZURICH_FB_POST_FROM", "2026-10-17")
		got, err := facebookPostFrom(zurich, []config.Jurisdiction{undated})
		if err != nil || !got.Equal(time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("got %v, %v", got, err)
		}
	})

	t.Run("a mistyped date is refused, not read as none", func(t *testing.T) {
		t.Setenv("ZURICH_FB_POST_FROM", "17.10.2026")
		if _, err := facebookPostFrom(zurich, []config.Jurisdiction{dated}); err == nil {
			t.Error("a mistyped FB_POST_FROM was accepted")
		}
	})

	t.Run("every jurisdiction dated is enough", func(t *testing.T) {
		got, err := facebookPostFrom(zurich, []config.Jurisdiction{dated})
		if err != nil || !got.IsZero() {
			t.Errorf("got %v, %v; want no platform date and no error", got, err)
		}
	})

	t.Run("one undated jurisdiction re-opens the backlog, so it is refused", func(t *testing.T) {
		_, err := facebookPostFrom(zurich, []config.Jurisdiction{dated, undated})
		if err == nil || !strings.Contains(err.Error(), "JURISDICTION_ZURICH_CANTON_POST_FROM") {
			t.Errorf("err = %v, want one naming the undated jurisdiction's variable", err)
		}
	})

	t.Run("nothing set is refused", func(t *testing.T) {
		if _, err := facebookPostFrom(zurich, []config.Jurisdiction{undated}); err == nil {
			t.Error("Facebook was allowed with no start date at all")
		}
	})
}
