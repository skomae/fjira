package ui

import (
	"fmt"
	"net/url"

	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/jira"
)

// OpenCreateIssueInBrowser opens the Jira create-issue modal in the user's
// browser, pre-populating project and board context where available.
// Uses the legacy CreateIssue!default.jspa endpoint which Atlassian Cloud
// redirects to the modern UI; on-prem Jira Server keeps the legacy form.
// Pass empty projectId / zero boardId to omit those query params.
func OpenCreateIssueInBrowser(api jira.Api, projectId string, boardId int) {
	jiraUrl := api.GetApiUrl()
	createUrl := fmt.Sprintf("%s/secure/CreateIssue!default.jspa", jiraUrl)
	params := url.Values{}
	if projectId != "" && projectId != MessageAll {
		params.Add("pid", projectId)
	}
	if boardId > 0 {
		params.Add("boardId", fmt.Sprintf("%d", boardId))
	}
	if len(params) > 0 {
		createUrl += "?" + params.Encode()
	}
	app.OpenLink(createUrl)
}
