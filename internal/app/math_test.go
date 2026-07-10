package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ScrollPageSize(t *testing.T) {
	tests := []struct {
		name    string
		screenY int
		want    int
	}{
		// visibleHeight = screenY-16, capped at screenY/2, floored at 5.
		{"tiny screen floors at 5", 10, 5},    // 10-16=-6 → clamp[1,5]=1 → floor 5
		{"small screen floors at 5", 20, 5},   // 20-16=4 → clamp[1,10]=4 → floor 5
		{"medium screen", 40, 20},             // 40-16=24 → capped at 40/2=20
		{"just above the cap", 44, 22},        // 44-16=28 → capped at 22
		{"large screen uses visible", 80, 40}, // 80-16=64 → capped at 80/2=40
		{"just above the floor", 26, 10},      // 26-16=10 → clamp[1,13]=10 → 10>5
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ScrollPageSize(tt.screenY))
		})
	}
}
