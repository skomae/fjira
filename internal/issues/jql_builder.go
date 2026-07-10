package issues

import (
	"fmt"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/ui"
	"strings"
)

// DefaultBoundedJql is used when no other restriction would be present in the
// generated query. Atlassian Cloud's /rest/api/3/search/jql endpoint rejects
// unbounded queries with HTTP 400 ("Unbounded JQL queries are not allowed here").
const DefaultBoundedJql = "created >= -30d"

// ORDER BY clauses the issue browser can toggle between with F9. Updated is
// DESC so the server returns the most recently updated issues within the single
// fetched page (rather than the oldest); the fuzzy-find list renders index 0 at
// the bottom, so newest-first from the API lands with the most recent at the
// bottom of the list.
const (
	OrderByStatus  = "ORDER BY status"
	OrderByUpdated = "ORDER BY updated DESC"
)

func BuildSearchIssuesJql(project *jira.Project, query string, status *jira.IssueStatus, user *jira.User, label string, excludedStatuses []*jira.IssueStatus, orderBy string) string {
	jql := ""
	if project != nil && project.Id != ui.MessageAll {
		jql = jql + fmt.Sprintf("project=%s", project.Id)
	}
	if orderBy == "" {
		orderBy = OrderByStatus
	}
	query = strings.TrimSpace(query)
	if query != "" {
		jql = jql + fmt.Sprintf(" AND summary~\"%s*\"", query)
	}
	if status != nil && status.Name != ui.MessageAll {
		jql = jql + fmt.Sprintf(" AND status=%s", status.Id)
	}
	if user != nil && user.DisplayName != ui.MessageAll {
		userId := user.AccountId
		if userId == "" {
			userId = user.Name
		}
		jql = jql + fmt.Sprintf(" AND assignee=%s", userId)
	}
	// TODO - would be safer to check the index of inserted all message, instead of checking it like this / same for all All checks
	if label != "" && label != ui.MessageAll {
		jql = jql + fmt.Sprintf(" AND labels=%s", label)
	}
	for _, excludedStatus := range excludedStatuses {
		if excludedStatus != nil && excludedStatus.Name != ui.MessageAll {
			jql = jql + fmt.Sprintf(" AND status!=%s", excludedStatus.Id)
		}
	}
	if query != "" && issueRegExp.MatchString(query) {
		jql = jql + fmt.Sprintf(" OR issuekey=\"%s\"", query)
	}
	jql = strings.TrimLeft(jql, " AND")
	if jql == "" {
		jql = DefaultBoundedJql
	}
	return fmt.Sprintf("%s %s", jql, orderBy)
}
