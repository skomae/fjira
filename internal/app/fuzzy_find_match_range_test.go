package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func Test_buildMatchTarget(t *testing.T) {
	// display: indices ->  0123456789...
	//          "PROJ-53   BUG   Fix login"
	display := "PROJ-53   BUG   Fix login"
	// matchable: key "PROJ-53" at [0,7), summary "Fix login" at [16,25)
	ranges := []MatchRange{{Start: 0, End: 7}, {Start: 16, End: 25}}

	target, mapping := buildMatchTarget(display, ranges)

	// target = key + " " + summary
	assert.Equal(t, "PROJ-53 Fix login", target)
	// mapping length equals target length
	assert.Equal(t, len(target), len(mapping))
	// the separator (index 7 in target) maps to -1
	assert.Equal(t, -1, mapping[7])
	// key bytes map to themselves
	for i := 0; i < 7; i++ {
		assert.Equal(t, i, mapping[i])
	}
	// summary bytes map to display offsets starting at 16
	assert.Equal(t, 16, mapping[8]) // first summary char after separator
	assert.Equal(t, 24, mapping[len(mapping)-1])
}

func Test_buildMatchTarget_emptyRanges(t *testing.T) {
	target, mapping := buildMatchTarget("anything", nil)
	assert.Equal(t, "", target)
	assert.Empty(t, mapping)
}

func Test_buildMatchTarget_outOfBoundsRangeSkipped(t *testing.T) {
	// a range past the end of the string is skipped without panicking
	target, mapping := buildMatchTarget("abc", []MatchRange{{Start: 0, End: 3}, {Start: 5, End: 10}})
	assert.Equal(t, "abc ", target) // second range contributes only the separator
	assert.Equal(t, []int{0, 1, 2, -1}, mapping)
}

// Dimmed rows must sort last while keeping the fuzzy-score order within each
// group, and match.Index must stay pointing at the right record.
func Test_rangeProvider_dimmedSortLast(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	CreateNewAppWithScreen(screen)

	// three records all matching "bug"; index 1 is dimmed (e.g. excluded status)
	records := []string{"AAA-1 login bug", "AAA-2 payment bug", "AAA-3 signup bug"}
	ranges := [][]MatchRange{
		{{0, len(records[0])}},
		{{0, len(records[1])}},
		{{0, len(records[2])}},
	}
	dimmed := []bool{false, true, false}
	provider := func(q string) ([]string, [][]MatchRange, []bool) { return records, ranges, dimmed }
	ff := NewFuzzyFindWithRangeProvider("t", provider)
	ff.SetDebounceDisabled(true)
	ff.SetQuery("bug")
	ff.ForceUpdate()

	matches := ff.Matches()
	assert.Len(t, matches, 3)
	// the dimmed record (index 1) must be last, regardless of its fuzzy score
	assert.Equal(t, 1, matches[len(matches)-1].Index)
	// the non-dimmed records come first
	assert.NotEqual(t, 1, matches[0].Index)
	assert.NotEqual(t, 1, matches[1].Index)
}
