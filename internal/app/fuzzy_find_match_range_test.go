package app

import (
	"testing"

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
