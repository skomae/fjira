package issues

import (
	"fmt"
	"strings"

	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/ui"
)

func BuildSearchIssuesJql(project *jira.Project, query string, status *jira.IssueStatus, user *jira.User, label string, excludeStatus *jira.IssueStatus) string {
	jql := ""
	if project != nil && project.Id != ui.MessageAll {
		jql = jql + fmt.Sprintf("project=%s", project.Id)
	}
	orderBy := "ORDER BY status"
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
	if excludeStatus != nil && excludeStatus.Name != ui.MessageAll {
		jql = jql + fmt.Sprintf(" AND status!=%s", excludeStatus.Id)
	}
	if query != "" && issueRegExp.MatchString(query) {
		jql = jql + fmt.Sprintf(" OR issuekey=\"%s\"", query)
	}
	return fmt.Sprintf("%s %s", strings.TrimLeft(jql, " AND"), orderBy)
}
