package jira

import (
	"encoding/json"
	"fmt"
	"strings"
)

type IssueType struct {
	Name string `json:"name"`
}

type Issue struct {
	Key    string      `json:"key"`
	Fields IssueFields `json:"Fields"`
	Id     string      `json:"id"`
}

type IssueFields struct {
	Summary     string  `json:"summary"`
	Project     Project `json:"project"`
	Description string  `json:"description,omitempty"`
	Reporter    struct {
		AccountId   string `json:"accountId"`
		DisplayName string `json:"displayName"`
	} `json:"reporter"`
	Assignee struct {
		AccountId   string `json:"accountId"`
		DisplayName string `json:"displayName"`
	} `json:"assignee"`
	Type struct {
		Name string `json:"name"`
	} `json:"issuetype"`
	Updated string `json:"updated"`
	Status  Status
	Comment struct {
		Comments   []Comment `json:"comments"`
		MaxResults int32     `json:"maxResults"`
		Total      int32     `json:"total"`
		StartAt    int32     `json:"startAt"`
	} `json:"comment"`
	Labels   []string `json:"labels"`
	Priority struct {
		Name string `json:"name"`
	} `json:"priority"`
	Created string `json:"created"`
	// Parent is the standard Jira parent link. For a story it is the epic; for
	// a sub-task it is the containing ticket. Modern Jira (v2/v3, Cloud and
	// recent Server) exposes both through this one field, distinguished by
	// Parent.Fields.Type.Name (== "Epic" for an epic). Zero-valued when unset.
	Parent struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string `json:"summary"`
			Type    struct {
				Name string `json:"name"`
			} `json:"issuetype"`
		} `json:"fields"`
	} `json:"parent"`
}

type descriptionUpdateRequestBody struct {
	Fields struct {
		Description string `json:"description"`
	} `json:"fields"`
}

const (
	GetJiraIssuePath = "/rest/api/2/issue/%s"
)

func (api *httpApi) GetIssueDetailed(id string) (*Issue, error) {
	body, err := api.jiraRequest("GET", fmt.Sprintf(GetJiraIssuePath, id), &nilParams{}, nil)
	if err != nil {
		return nil, err
	}
	var jiraIssue Issue
	if err := json.Unmarshal(body, &jiraIssue); err != nil {
		return nil, ErrSearchDeserialize
	}
	return &jiraIssue, nil
}

func (api *httpApi) DoUpdateDescription(issueId string, description string) error {
	request := &descriptionUpdateRequestBody{}
	request.Fields.Description = description
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return err
	}
	_, err = api.jiraRequest("PUT", fmt.Sprintf(GetJiraIssuePath, issueId), &nilParams{}, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}
	return nil
}
