package i18n

import (
	"strings"

	"golang.org/x/text/language"
)

// maxAcceptLanguageLen caps how much of the Accept-Language header is parsed.
// Browsers send a few dozen bytes.
const maxAcceptLanguageLen = 512

// ParseAcceptLanguage parses an Accept-Language header value into language
// tags, most preferred first. Only the comma-separated entries that fit whole
// within the first maxAcceptLanguageLen bytes are parsed.
func ParseAcceptLanguage(header string) []language.Tag {
	tags, _, _ := language.ParseAcceptLanguage(truncateAcceptLanguage(header))
	return tags
}

// truncateAcceptLanguage drops entries from the end of header until it fits in
// maxAcceptLanguageLen bytes. Whole entries are dropped because one cut in the
// middle would make language.ParseAcceptLanguage reject the entire header.
func truncateAcceptLanguage(header string) string {
	if len(header) <= maxAcceptLanguageLen {
		return header
	}
	// Including the byte at the limit keeps every entry when it is a comma.
	if i := strings.LastIndexByte(header[:maxAcceptLanguageLen+1], ','); i >= 0 {
		return header[:i]
	}
	return ""
}
