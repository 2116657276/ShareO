package service

import (
	"regexp"
	"testing"
)

func TestParseHashtags(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantCount    int
		wantContains []string
	}{
		{"empty", "", 0, nil},
		{"no_tags", "这是一段普通文字", 0, nil},
		{"single_chinese", "#风光 拍摄于今天", 1, []string{"风光"}},
		{"single_english", "great #landscape photo", 1, []string{"landscape"}},
		{"multiple", "#风光 #街拍 #黑白摄影", 3, []string{"风光", "街拍", "黑白摄影"}},
		{"with_numbers", "shot with #iPhone15 #35mm", 2, []string{"35mm"}},
		{"dedup", "#风光 and #风光 again", 1, []string{"风光"}},
		{"underscores", "#street_photo #golden_hour", 2, []string{"street_photo", "golden_hour"}},
		{"at_end", "beautiful #sunset", 1, []string{"sunset"}},
		{"at_start", "#sunrise is beautiful", 1, []string{"sunrise"}},
		{"dot_separated", "#tag1.#tag2", 2, []string{"tag1", "tag2"}},
		{"comma_separated", "#tag1,#tag2", 2, []string{"tag1", "tag2"}},
		{"single_char", "#a", 1, []string{"a"}},
		{"japanese", "#東京カメラ部", 1, []string{"東京カメラ部"}},
		{"korean", "#서울풍경", 1, []string{"서울풍경"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseHashtags(tt.content)
			if len(got) != tt.wantCount {
				t.Errorf("ParseHashtags(%q) len=%d, want len=%d: %v", tt.content, len(got), tt.wantCount, got)
				return
			}
			for _, want := range tt.wantContains {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ParseHashtags(%q) = %v, should contain %q", tt.content, got, want)
				}
			}
		})
	}
}

func TestParseHashtags_EdgeCases(t *testing.T) {
	// "not#atag" — regex matches #atag starting at # position
	got := ParseHashtags("not#atag")
	if len(got) != 1 || got[0] != "atag" {
		t.Errorf("ParseHashtags(not#atag) = %v, want [atag]", got)
	}

	// 51-char tag — accepted by regex, filtered by len(name) <= 50
	longTag := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 50 a's
	got = ParseHashtags("#" + longTag)
	if len(got) != 1 || got[0] != longTag {
		t.Errorf("50-char tag should be accepted: got %v", got)
	}

	// 51-char tag — filtered out
	got = ParseHashtags("#" + longTag + "b")
	if len(got) != 0 {
		t.Errorf("51-char tag should be filtered: got %v", got)
	}

	// Empty hashtag "# " (space after hash)
	got = ParseHashtags("# ")
	if len(got) != 0 {
		t.Errorf("empty hashtag should return nil: got %v", got)
	}
}

func TestHashtagRegex_CasePreservation(t *testing.T) {
	// Verify regex preserves exact case from input
	re := regexp.MustCompile(`#([\p{L}\p{N}_]+)`)
	input := "check #CamelCase here"
	matches := re.FindAllStringSubmatch(input, -1)
	if len(matches) != 1 || matches[0][1] != "CamelCase" {
		t.Errorf("regex case: input=%q, got=%v", input, matches)
	}
}
