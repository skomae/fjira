package app

import (
	"fmt"
	"time"
)

// JiraTimestampLayout is the timestamp format Jira returns for date/time
// fields such as "updated" and "created", e.g. 2026-07-10T14:30:00.000-0400.
const JiraTimestampLayout = "2006-01-02T15:04:05.000-0700"

// FormatRelativeTime renders a Jira timestamp as a friendly relative string
// ("2 hours ago", "yesterday", "last week"). now is injected so the function
// stays pure and deterministically testable. An empty or unparseable timestamp
// yields "" so the caller degrades gracefully rather than panicking.
func FormatRelativeTime(ts string, now time.Time) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(JiraTimestampLayout, ts)
	if err != nil {
		return ""
	}
	d := max(now.Sub(t), 0)
	switch {
	case d < time.Minute:
		return "just now"
	case d < 2*time.Minute:
		return "1 min ago"
	case d < time.Hour:
		return fmt.Sprintf("%d mins ago", int(d.Minutes()))
	case d < 2*time.Hour:
		return "1 hour ago"
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	case d < 14*24*time.Hour:
		return "last week"
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/(24*7)))
	case d < 60*24*time.Hour:
		return "last month"
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d months ago", int(d.Hours()/(24*30)))
	case d < 2*365*24*time.Hour:
		return "last year"
	default:
		return fmt.Sprintf("%d years ago", int(d.Hours()/(24*365)))
	}
}
