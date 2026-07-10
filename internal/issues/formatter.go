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
	return fmt.Sprintf("%10s %"+strconv.Itoa(typeColWidth+ui.TableColumnPadding)+"s %"+strconv.Itoa(summaryColWidth+ui.TableColumnPadding)+"s %"+strconv.Itoa(statusColWidth+4+ui.TableColumnPadding)+"s %"+strconv.Itoa(assigneeColWidth+2+ui.TableColumnPadding)+"s %s",
		issue.Key,
		strings.ToUpper(issue.Fields.Type.Name[:typeCut]),
		issue.Fields.Summary[:summaryCut],
		fmt.Sprintf("[%s]", strings.ToUpper(issue.Fields.Status.Name[:statusCut])),
		fmt.Sprintf("- %s", assignee[:assigneeCut]),
		formatRelativeTime(issue.Fields.Updated, now))
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
