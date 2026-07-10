package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_FormatRelativeTime(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	ts := func(d time.Duration) string {
		return now.Add(-d).Format(JiraTimestampLayout)
	}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty string", "", ""},
		{"unparseable", "not-a-date", ""},
		{"just now", ts(10 * time.Second), "just now"},
		{"one minute", ts(90 * time.Second), "1 min ago"},
		{"minutes", ts(5 * time.Minute), "5 mins ago"},
		{"one hour", ts(75 * time.Minute), "1 hour ago"},
		{"hours", ts(3 * time.Hour), "3 hours ago"},
		{"yesterday", ts(26 * time.Hour), "yesterday"},
		{"days", ts(3 * 24 * time.Hour), "3 days ago"},
		{"last week", ts(8 * 24 * time.Hour), "last week"},
		{"weeks", ts(20 * 24 * time.Hour), "2 weeks ago"},
		{"last month", ts(40 * 24 * time.Hour), "last month"},
		{"months", ts(90 * 24 * time.Hour), "3 months ago"},
		{"last year", ts(400 * 24 * time.Hour), "last year"},
		{"years", ts(800 * 24 * time.Hour), "2 years ago"},
		{"future clamps to just now", now.Add(time.Hour).Format(JiraTimestampLayout), "just now"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatRelativeTime(tt.in, now))
		})
	}
}

func Test_FormatAbsoluteTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty string", "", ""},
		{"unparseable", "not-a-date", ""},
		{"utc", "2026-07-10T10:00:00.000+0000", "10 Jul 2026 10:00 AM +0000"},
		{"pm", "2026-06-04T22:35:00.000+0200", "4 Jun 2026 10:35 PM +0200"},
		{"am with offset", "2026-06-04T10:35:00.000+0200", "4 Jun 2026 10:35 AM +0200"},
		{"negative offset", "2026-07-10T14:30:00.000-0400", "10 Jul 2026 2:30 PM -0400"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatAbsoluteTime(tt.in))
		})
	}
}
