package jira

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/google/go-querystring/query"
)

func (api *httpApi) jiraRequest(method string, restPath string, queryParams interface{}, reqBody io.Reader) ([]byte, error) {
	u, err := api.jiraRequestUrl(restPath, queryParams)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, u, reqBody)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	if err != nil {
		return nil, err
	}
	response, err := api.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)
	if response.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
		return nil, newJiraError(response.Status, req.Method, req.URL.Path, errBody)
	}
	body, _ := io.ReadAll(response.Body)
	return body, nil
}

// maxErrorBodyBytes caps how much of an error response is read and reported.
const maxErrorBodyBytes = 4 * 1024

// maxRawSnippetChars caps the raw (non-JSON) body snippet included in an error.
const maxRawSnippetChars = 300

type jiraErrorBody struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

// newJiraError builds "jira error, status: <status> - request: <METHOD> <path>"
// followed by ": <explanation>" when the response body has one. Only the path is
// reported (no query, userinfo or headers), so no credentials can leak.
func newJiraError(status, method, urlPath string, body []byte) error {
	msg := fmt.Sprintf("jira error, status: %s - request: %s %s", status, method, urlPath)
	if detail := jiraErrorDetail(body); detail != "" {
		msg += ": " + detail
	}
	return errors.New(msg)
}

func jiraErrorDetail(body []byte) string {
	var parsed jiraErrorBody
	if err := json.Unmarshal(body, &parsed); err == nil {
		var parts []string
		for _, m := range parsed.ErrorMessages {
			if m = strings.TrimSpace(m); m != "" {
				parts = append(parts, m)
			}
		}
		fields := make([]string, 0, len(parsed.Errors))
		for k := range parsed.Errors {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		for _, k := range fields {
			parts = append(parts, fmt.Sprintf("%s: %s", k, strings.TrimSpace(parsed.Errors[k])))
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	snippet := strings.Join(strings.Fields(string(body)), " ")
	if r := []rune(snippet); len(r) > maxRawSnippetChars {
		snippet = string(r[:maxRawSnippetChars]) + "..."
	}
	return snippet
}

func (api *httpApi) jiraRequestUrl(restPath string, queryParams interface{}) (string, error) {
	queryParamsValues, err := query.Values(queryParams)
	if err != nil {
		return "", err
	}
	u := api.restUrl.ResolveReference(&url.URL{Path: path.Join(api.restUrl.Path, restPath), RawQuery: queryParamsValues.Encode()})
	return u.String(), err
}
