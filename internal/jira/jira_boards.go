package jira

import (
	"encoding/json"
	"fmt"
)

type BoardItem struct {
	Id   int    `json:"id"`
	Self string `json:"self"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type BoardsResponse struct {
	MaxResults int         `json:"maxResults"`
	StartAt    int         `json:"startAt"`
	Total      int         `json:"total"`
	IsLast     bool        `json:"isLast"`
	Values     []BoardItem `json:"values"`
}

type BoardConfiguration struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Self     string `json:"self"`
	Location struct {
		Type string `json:"type"`
		Key  string `json:"key"`
		Id   string `json:"id"`
		Self string `json:"self"`
		Name string `json:"name"`
	} `json:"location"`
	Filter struct {
		Id   string `json:"id"`
		Self string `json:"self"`
	} `json:"filter"`
	SubQuery struct {
		Query string `json:"query"`
	} `json:"subQuery"`
	ColumnConfig struct {
		Columns []struct {
			Name     string `json:"name"`
			Statuses []struct {
				Id   string `json:"id"`
				Self string `json:"self"`
			} `json:"statuses"`
		} `json:"columns"`
		ConstraintType string `json:"constraintType"`
	} `json:"columnConfig"`
	Ranking struct {
		RankCustomFieldId int `json:"rankCustomFieldId"`
	} `json:"ranking"`
}

const (
	FindAllBoardsUrl          = "/rest/agile/1.0/board"
	FindBoardConfigurationUrl = "/rest/agile/1.0/board/%d/configuration"
	GetBoardIssuesUrl         = "/rest/agile/1.0/board/%d/issue"
)

type findBoardsQueryParams struct {
	ProjectKeyOrId string `url:"projectKeyOrId"`
	StartAt        int    `url:"startAt"`
}

type getBoardIssuesQueryParams struct {
	StartAt    int32  `url:"startAt"`
	MaxResults int32  `url:"maxResults"`
	Fields     string `url:"fields"`
	JQL        string `url:"jql,omitempty"`
}

func (api *httpApi) FindBoards(projectKeyOrId string) ([]BoardItem, error) {
	params := &findBoardsQueryParams{ProjectKeyOrId: projectKeyOrId, StartAt: 0}
	var boards []BoardItem
	for {
		resultBytes, err := api.jiraRequest("GET", FindAllBoardsUrl, params, nil)
		if err != nil {
			return nil, err
		}
		var result BoardsResponse
		err = json.Unmarshal(resultBytes, &result)
		if err != nil {
			return nil, err
		}
		if cap(boards) == 0 {
			boards = make([]BoardItem, 0, result.Total)
		}
		boards = append(boards, result.Values...)

		if result.IsLast {
			break
		}
		params.StartAt += result.MaxResults
	}
	return boards, nil
}

func (api *httpApi) GetBoardConfiguration(boardId int) (*BoardConfiguration, error) {
	resultBytes, err := api.jiraRequest("GET", fmt.Sprintf(FindBoardConfigurationUrl, boardId), &nilParams{}, nil)
	if err != nil {
		return nil, err
	}
	var result BoardConfiguration
	err = json.Unmarshal(resultBytes, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetBoardProjects fetches the projects associated with a board by its ID.
func (api *httpApi) GetBoardProjects(boardId int) ([]Project, error) {
	url := fmt.Sprintf("/rest/agile/1.0/board/%d/project", boardId)
	resultBytes, err := api.jiraRequest("GET", url, &nilParams{}, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Values []Project `json:"values"`
	}
	err = json.Unmarshal(resultBytes, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Values, nil
}

// GetBoardIssues fetches issues for a board using the Agile API with optional JQL filtering
func (api *httpApi) GetBoardIssues(boardId int, page int32, pageSize int32, jql string) ([]Issue, int32, int32, error) {
	params := &getBoardIssuesQueryParams{
		StartAt:    page * pageSize,
		MaxResults: pageSize,
		Fields:     "id,key,summary,issuetype,project,reporter,status,assignee",
		JQL:        jql,
	}

	resultBytes, err := api.jiraRequest("GET", fmt.Sprintf(GetBoardIssuesUrl, boardId), params, nil)
	if err != nil {
		return nil, -1, pageSize, err
	}

	var result struct {
		Issues     []Issue `json:"issues"`
		Total      int32   `json:"total"`
		MaxResults int32   `json:"maxResults"`
		StartAt    int32   `json:"startAt"`
	}

	err = json.Unmarshal(resultBytes, &result)
	if err != nil {
		return nil, -1, pageSize, err
	}

	return result.Issues, result.Total, result.MaxResults, nil
}
