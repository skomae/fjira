package jira

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func Test_httpApi_jiraRequest_should_return_error_when_http_client_error(t *testing.T) {
	// given
	api := &httpApi{
		client: &http.Client{
			Transport: &authInterceptor{core: http.DefaultTransport, token: "test"},
		},
		restUrl: &url.URL{},
	}

	// when
	_, err := api.jiraRequest("POST", "test", struct{}{}, nil)

	// then
	assert.NotNil(t, err)
}

func Test_httpApi_jiraRequest_should_return_error_when_invalid_params(t *testing.T) {
	// given
	api := &httpApi{
		client: &http.Client{
			Transport: &authInterceptor{core: http.DefaultTransport, token: "test"},
		},
		restUrl: &url.URL{},
	}

	// when
	_, err := api.jiraRequest("POST", "test", "invalid params", nil)

	// then
	assert.NotNil(t, err)
}

func Test_jiraRequest_combinePaths(t *testing.T) {
	tests := []struct {
		name     string
		apiUrl   string
		restPath string
		wanted   string
	}{
		{"should add label without error", "http://localhost", "/api1", "http://localhost/api1"},
		{"should add label without error", "http://localhost/", "/api1", "http://localhost/api1"},
		{"should add label without error", "http://localhost/jira-api-v2", "/api1", "http://localhost/jira-api-v2/api1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.apiUrl)
			api := &httpApi{restUrl: u}

			result, _ := api.jiraRequestUrl(tt.restPath, nil)

			assert.Equal(t, tt.wanted, result)
		})
	}
}

func Test_httpApi_jiraRequest_error_body(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		notWant string
	}{
		{"errorMessages only", `{"errorMessages":["Please enter an effort estimate.","Please enter a description."],"errors":{}}`,
			"Please enter an effort estimate.; Please enter a description.", ""},
		{"errors map only", `{"errorMessages":[],"errors":{"summary":"Summary is required","assignee":"User does not exist"}}`,
			"assignee: User does not exist; summary: Summary is required", ""},
		{"both", `{"errorMessages":["Transition failed"],"errors":{"timetracking":"Estimate required"}}`,
			"Transition failed; timetracking: Estimate required", ""},
		{"non-JSON body", "  <html>\n  Bad   gateway  </html> ", ": <html> Bad gateway </html>", "\n"},
		{"empty body", "", "", "transitions: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			u, _ := url.Parse(server.URL)
			api := &httpApi{
				client:  &http.Client{Transport: &authInterceptor{core: http.DefaultTransport, token: "secret-token"}},
				restUrl: u,
			}

			_, err := api.jiraRequest("POST", "/rest/api/2/issue/TEST-1/transitions", &nilParams{}, strings.NewReader("{}"))

			assert.Error(t, err)
			prefix := "jira error, status: 400 Bad Request - request: POST /rest/api/2/issue/TEST-1/transitions"
			assert.True(t, strings.HasPrefix(err.Error(), prefix), err.Error())
			if tt.want != "" {
				assert.Contains(t, err.Error(), tt.want)
			}
			if tt.notWant != "" {
				assert.NotContains(t, err.Error(), tt.notWant)
			}
			assert.NotContains(t, err.Error(), "secret-token")
			if tt.body == "" {
				assert.Equal(t, prefix, err.Error())
			}
		})
	}
}

func Test_jiraErrorDetail_should_truncate_long_raw_body(t *testing.T) {
	detail := jiraErrorDetail([]byte(strings.Repeat("x", 5000)))

	assert.LessOrEqual(t, len(detail), maxRawSnippetChars+3)
	assert.True(t, strings.HasSuffix(detail, "..."))
}
