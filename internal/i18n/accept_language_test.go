package i18n

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTruncateAcceptLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"short header is kept", "fr-FR,fr;q=0.9,en;q=0.8", "fr-FR,fr;q=0.9,en;q=0.8"},
		{"header at the limit is kept", strings.Repeat("en,", 170) + "en", strings.Repeat("en,", 170) + "en"},
		{"entry cut by the limit is dropped", strings.Repeat("de-DE;q=0.5,", 50), strings.Repeat("de-DE;q=0.5,", 41) + "de-DE;q=0.5"},
		{"comma right after the limit keeps every entry before it", strings.Repeat("en,", 171) + "fr-FR", strings.Repeat("en,", 170) + "en"},
		{"single entry over the limit is dropped", strings.Repeat("a", 600), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateAcceptLanguage(tt.header)
			require.Equal(t, tt.want, got)
			require.LessOrEqual(t, len(got), maxAcceptLanguageLen)
		})
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	tags := func(header string) []string {
		var out []string
		for _, tag := range ParseAcceptLanguage(header) {
			out = append(out, tag.String())
		}
		return out
	}

	require.Equal(t, []string{"fr-FR", "fr", "en"}, tags("fr-FR,fr;q=0.9,en;q=0.8"))
	require.Empty(t, tags(""))
	// Cutting through "de-DE;q=0.5" would make x/text reject the whole header.
	require.Len(t, tags(strings.Repeat("de-DE;q=0.5,", 50)), 42)
}
