package keywords

import (
	"reflect"
	"testing"

	"procom/internal/config"
)

func TestMatcherDetectsCaseInsensitiveWholeWordMatches(t *testing.T) {
	t.Parallel()

	matcher := NewMatcher([]config.KeywordConfig{{
		Phrase:         "GO",
		HighlightColor: "#22C55E",
		WholeWord:      true,
	}})

	keywords, matches := matcher.Match("We go now, then gopher later.")

	if !reflect.DeepEqual(keywords, []string{"GO"}) {
		t.Fatalf("keywords = %v, want [GO]", keywords)
	}
	if len(matches) != 1 {
		t.Fatalf("matches len = %d, want 1", len(matches))
	}
	if matches[0].Start != 3 || matches[0].End != 5 {
		t.Fatalf("match range = [%d,%d], want [3,5]", matches[0].Start, matches[0].End)
	}
}

func TestMatcherRespectsCaseSensitiveRules(t *testing.T) {
	t.Parallel()

	matcher := NewMatcher([]config.KeywordConfig{{
		Phrase:         "LX",
		HighlightColor: "#EAB308",
		CaseSensitive:  true,
	}})

	keywords, matches := matcher.Match("lx then LX")
	if !reflect.DeepEqual(keywords, []string{"LX"}) {
		t.Fatalf("keywords = %v, want [LX]", keywords)
	}
	if len(matches) != 1 {
		t.Fatalf("matches len = %d, want 1", len(matches))
	}
}
