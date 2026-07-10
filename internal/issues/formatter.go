package issues

import (
	"fmt"
	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/ui"
	"strconv"
	"strings"
	"time"
)

// jiraUpdatedLayout is the timestamp format Jira returns for the "updated"
// field, e.g. 2026-07-10T14:30:00.000-0400.
const jiraUpdatedLayout = "2006-01-02T15:04:05.000-0700"

// formatRelativeTime renders a Jira "updated" timestamp as a friendly relative
// string ("2 hours ago", "yesterday", "last week"). now is injected so the
// function stays pure and deterministically testable. An empty or unparseable
// timestamp yields "" so the caller degrades gracefully rather than panicking.
func formatRelativeTime(updated string, now time.Time) string {
	if updated == "" {
		return ""
	}
	t, err := time.Parse(jiraUpdatedLayout, updated)
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

func FormatJiraIssue(issue *jira.Issue) string {
	return fmt.Sprintf("%s %s [%s] - %s",
		issue.Key,
		issue.Fields.Summary,
		issue.Fields.Status.Name,
		FormatAssignee(issue))
}

func FormatJiraIssueTable(issue *jira.Issue, summaryColWidth int, statusColWidth int, typeColWidth int, assigneeColWidth int, now time.Time) string {
	row, _ := formatJiraIssueTableWithRanges(issue, summaryColWidth, statusColWidth, typeColWidth, assigneeColWidth, now)
	return row
}

// formatJiraIssueTableWithRanges formats one issue row and returns, alongside
// it, the byte ranges of the matchable columns (the issue key and the summary).
// Fuzzy matching/highlighting is scoped to those ranges so the type, status,
// assignee, and last-updated columns are never matched or highlighted. Offsets
// are tracked during assembly rather than located afterwards, so summary text
// that happens to recur elsewhere can't be mismatched.
func formatJiraIssueTableWithRanges(issue *jira.Issue, summaryColWidth int, statusColWidth int, typeColWidth int, assigneeColWidth int, now time.Time) (string, []app.MatchRange) {
	assignee := issue.Fields.Assignee.DisplayName
	if assignee == "" {
		assignee = ui.MessageUnassigned
	}
	summaryColWidth = app.MinInt(summaryColWidth, ui.MaxSummaryColWidth)
	summaryCut := app.MinInt(summaryColWidth, len(issue.Fields.Summary))
	statusColWidth = app.MinInt(statusColWidth, ui.MaxStatusColWidth)
	statusCut := app.MinInt(statusColWidth, len(issue.Fields.Status.Name))
	typeColWidth = app.MinInt(typeColWidth, ui.MaxTypeColWidth)
	typeCut := app.MinInt(typeColWidth, len(issue.Fields.Type.Name))
	// Clamp the assignee column so the trailing "last updated" date lines up
	// vertically instead of drifting with each name's length.
	assigneeColWidth = app.MinInt(assigneeColWidth, ui.MaxAssigneeColWidth)
	assigneeCut := app.MinInt(assigneeColWidth, len(assignee))

	keyCol := fmt.Sprintf("%10s", issue.Key)
	typeCol := fmt.Sprintf("%"+strconv.Itoa(typeColWidth+ui.TableColumnPadding)+"s", strings.ToUpper(issue.Fields.Type.Name[:typeCut]))
	summaryCol := fmt.Sprintf("%"+strconv.Itoa(summaryColWidth+ui.TableColumnPadding)+"s", issue.Fields.Summary[:summaryCut])
	statusCol := fmt.Sprintf("%"+strconv.Itoa(statusColWidth+4+ui.TableColumnPadding)+"s", fmt.Sprintf("[%s]", strings.ToUpper(issue.Fields.Status.Name[:statusCut])))
	assigneeCol := fmt.Sprintf("%"+strconv.Itoa(assigneeColWidth+2+ui.TableColumnPadding)+"s", fmt.Sprintf("- %s", assignee[:assigneeCut]))
	dateCol := formatRelativeTime(issue.Fields.Updated, now)

	// Assemble with single-space separators, tracking byte offsets so we can
	// mark the matchable ranges. right-aligned padding means the value sits at
	// the end of its column, so the value starts where the trimmed value begins.
	var b strings.Builder
	var ranges []app.MatchRange
	// matchFrom marks a matchable range from a byte offset within the value
	// (offset 0 = whole trimmed value). -1 means the column is not matchable.
	writeCol := func(col string, matchFrom int) {
		start := b.Len()
		b.WriteString(col)
		if matchFrom >= 0 {
			// Skip leading padding so the range covers only the value.
			valueStart := start + (len(col) - len(strings.TrimLeft(col, " ")))
			ranges = append(ranges, app.MatchRange{Start: valueStart + matchFrom, End: b.Len()})
		}
	}
	// Only the numeric issue number is matchable on the key (per the spec:
	// "matching numbers on the issue number") — so an alphabetic query never
	// lands on the project prefix (e.g. the S in COINS for a "skip" search).
	writeCol(keyCol, issueKeyNumberOffset(issue.Key))
	b.WriteByte(' ')
	writeCol(typeCol, -1)
	b.WriteByte(' ')
	writeCol(summaryCol, 0)
	b.WriteByte(' ')
	writeCol(statusCol, -1)
	b.WriteByte(' ')
	writeCol(assigneeCol, -1)
	b.WriteByte(' ')
	writeCol(dateCol, -1)
	return b.String(), ranges
}

// issueKeyNumberOffset returns the byte offset within an issue key at which the
// numeric issue number begins — the run after the final '-' (e.g. 6 for
// "COINS-115", pointing at "115"). Falls back to 0 (whole key matchable) if the
// key has no '-' or no trailing digits, so unusual keys stay searchable.
func issueKeyNumberOffset(key string) int {
	dash := strings.LastIndex(key, "-")
	if dash < 0 || dash+1 >= len(key) {
		return 0
	}
	for _, r := range key[dash+1:] {
		if r < '0' || r > '9' {
			return 0
		}
	}
	return dash + 1
}

func FormatJiraIssues(issues []jira.Issue) []string {
	formatted := make([]string, 0, len(issues))
	summaryColWidth := findIssueColumnSize(&issues, func(i jira.Issue) string {
		return i.Fields.Summary
	})
	statusColWidth := findIssueColumnSize(&issues, func(i jira.Issue) string {
		return i.Fields.Status.Name
	})
	typeColWidth := findIssueColumnSize(&issues, func(i jira.Issue) string {
		return i.Fields.Type.Name
	})
	assigneeColWidth := findIssueColumnSize(&issues, func(i jira.Issue) string {
		if i.Fields.Assignee.DisplayName == "" {
			return ui.MessageUnassigned
		}
		return i.Fields.Assignee.DisplayName
	})
	now := time.Now()
	for _, issue := range issues {
		formatted = append(formatted, FormatJiraIssueTable(&issue, summaryColWidth, statusColWidth, typeColWidth, assigneeColWidth, now))
	}
	return formatted
}

// FormatJiraIssuesWithRanges formats the issues and returns, index-aligned with
// the rows, the matchable byte ranges (key + summary) for each. Fed to the
// range-aware fuzzy finder so matching/highlighting ignore the other columns.
func FormatJiraIssuesWithRanges(issues []jira.Issue) ([]string, [][]app.MatchRange) {
	formatted := make([]string, 0, len(issues))
	ranges := make([][]app.MatchRange, 0, len(issues))
	summaryColWidth := findIssueColumnSize(&issues, func(i jira.Issue) string {
		return i.Fields.Summary
	})
	statusColWidth := findIssueColumnSize(&issues, func(i jira.Issue) string {
		return i.Fields.Status.Name
	})
	typeColWidth := findIssueColumnSize(&issues, func(i jira.Issue) string {
		return i.Fields.Type.Name
	})
	assigneeColWidth := findIssueColumnSize(&issues, func(i jira.Issue) string {
		if i.Fields.Assignee.DisplayName == "" {
			return ui.MessageUnassigned
		}
		return i.Fields.Assignee.DisplayName
	})
	now := time.Now()
	for _, issue := range issues {
		row, rr := formatJiraIssueTableWithRanges(&issue, summaryColWidth, statusColWidth, typeColWidth, assigneeColWidth, now)
		formatted = append(formatted, row)
		ranges = append(ranges, rr)
	}
	return formatted, ranges
}

func FormatAssignee(issue *jira.Issue) string {
	assignee := issue.Fields.Assignee.DisplayName
	if assignee == "" {
		assignee = ui.MessageUnassigned
	}
	return assignee
}

func findIssueColumnSize(items *[]jira.Issue, colSupplier func(issue jira.Issue) string) int {
	max := 0
	for _, item := range *items {
		current := colSupplier(item)
		if max < len(current) {
			max = len(current)
		}
	}
	return max
}
