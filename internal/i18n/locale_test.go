package i18n

import (
	"testing"

	"golang.org/x/text/language"
)

func TestMatchTag(t *testing.T) {
	store := NewLocaleStore()
	for _, code := range []string{"en-US", "fr-FR", "zh-CN", "zh-TW"} {
		store.Locales[code] = &Locale{Code: code}
	}

	tests := []struct {
		header string
		want   string
	}{
		{"en-NZ,fr-FR;q=0.5", "en-US"},
		{"fr", "fr-FR"},
		{"zh-TW,en;q=0.5", "zh-TW"},
		{"zh-HK", "zh-CN"},
		{"de-DE", "en-US"},
		{"", "en-US"},
	}
	for _, tt := range tests {
		tags, _, _ := language.ParseAcceptLanguage(tt.header)
		if got := store.MatchTag(tags); got != tt.want {
			t.Errorf("MatchTag(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}
