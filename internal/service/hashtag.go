package service

import (
	"regexp"
	"strings"
)

// hashtagRE matches #topic names in content.
// Matches: #风光, #街拍, #黑白摄影, #35mm, #iPhone摄影
// Stops at spaces, punctuation, and common delimiters.
var hashtagRE = regexp.MustCompile(`#([\p{L}\p{N}_]+)`)

// ParseHashtags extracts unique #topic names from text.
// Returns deduplicated, order-preserving topic names.
func ParseHashtags(content string) []string {
	matches := hashtagRE.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	var tags []string
	for _, m := range matches {
		name := strings.ToLower(m[1])
		if !seen[name] && len(name) >= 1 && len(name) <= 50 {
			seen[name] = true
			tags = append(tags, name)
		}
	}
	return tags
}
