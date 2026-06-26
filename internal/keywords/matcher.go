package keywords

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"procom/internal/config"
)

// Rule defines one configurable keyword matcher.
type Rule struct {
	Phrase         string
	HighlightColor string
	CaseSensitive  bool
	WholeWord      bool
	OSCAddress     string
	OSCArguments   []string
}

// Match describes one keyword match in rune offsets.
type Match struct {
	Phrase string
	Color  string
	Start  int
	End    int
}

// Matcher applies configured keyword rules to finalized transcript text.
type Matcher struct {
	rules []Rule
}

// NewMatcher compiles keyword configuration into a matcher.
func NewMatcher(configs []config.KeywordConfig) *Matcher {
	rules := make([]Rule, 0, len(configs))
	for _, current := range configs {
		rules = append(rules, Rule{
			Phrase:         current.Phrase,
			HighlightColor: current.HighlightColor,
			CaseSensitive:  current.CaseSensitive,
			WholeWord:      current.WholeWord,
			OSCAddress:     current.OSCAddress,
			OSCArguments:   append([]string(nil), current.OSCArguments...),
		})
	}
	return &Matcher{rules: rules}
}

// Match returns detected keyword phrases and highlight ranges for the provided text.
func (m *Matcher) Match(text string) ([]string, []Match) {
	if m == nil || len(m.rules) == 0 || text == "" {
		return nil, nil
	}

	seen := make(map[string]struct{})
	keywords := make([]string, 0)
	matches := make([]Match, 0)

	for _, rule := range m.rules {
		found := findMatches(text, rule)
		if len(found) == 0 {
			continue
		}
		if _, ok := seen[rule.Phrase]; !ok {
			seen[rule.Phrase] = struct{}{}
			keywords = append(keywords, rule.Phrase)
		}
		matches = append(matches, found...)
	}

	sort.SliceStable(matches, func(left, right int) bool {
		if matches[left].Start == matches[right].Start {
			return matches[left].End < matches[right].End
		}
		return matches[left].Start < matches[right].Start
	})

	return keywords, matches
}

func findMatches(text string, rule Rule) []Match {
	if strings.TrimSpace(rule.Phrase) == "" {
		return nil
	}

	searchText := text
	searchPhrase := rule.Phrase
	if !rule.CaseSensitive {
		searchText = strings.ToLower(text)
		searchPhrase = strings.ToLower(rule.Phrase)
	}

	byteOffsets := runeByteOffsets(text)
	results := make([]Match, 0)
	searchFrom := 0
	phraseBytes := len(searchPhrase)
	for {
		index := strings.Index(searchText[searchFrom:], searchPhrase)
		if index < 0 {
			break
		}
		startByte := searchFrom + index
		endByte := startByte + phraseBytes
		startRune := byteToRuneIndex(byteOffsets, startByte)
		endRune := byteToRuneIndex(byteOffsets, endByte)

		if rule.WholeWord && !isWholeWord(text, startByte, endByte) {
			searchFrom = endByte
			continue
		}

		results = append(results, Match{
			Phrase: rule.Phrase,
			Color:  rule.HighlightColor,
			Start:  startRune,
			End:    endRune,
		})
		searchFrom = endByte
	}

	return results
}

func runeByteOffsets(text string) []int {
	offsets := make([]int, 0, utf8.RuneCountInString(text)+1)
	for index := range text {
		offsets = append(offsets, index)
	}
	offsets = append(offsets, len(text))
	return offsets
}

func byteToRuneIndex(offsets []int, target int) int {
	index := sort.SearchInts(offsets, target)
	if index >= len(offsets) {
		return len(offsets) - 1
	}
	return index
}

func isWholeWord(text string, startByte int, endByte int) bool {
	if startByte > 0 {
		previous, _ := utf8.DecodeLastRuneInString(text[:startByte])
		if isWordRune(previous) {
			return false
		}
	}
	if endByte < len(text) {
		next, _ := utf8.DecodeRuneInString(text[endByte:])
		if isWordRune(next) {
			return false
		}
	}
	return true
}

func isWordRune(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsDigit(value) || value == '_'
}
