package jira

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func Test_httpJiraApi_DoTransition(t *testing.T) {
	type args struct {
		issueId    string
		transition *IssueTransition
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"should do transition without error",
			args{transition: &IssueTransition{
				Id:   "test",
				Name: "test",
				To: struct {
					StatusUrl string `json:"self"`
					StatusId  string `json:"id"`
					Name      string `json:"name"`
				}(struct {
					StatusUrl string
					StatusId  string
					Name      string
				}{"test2", "test2", "test2"}),
			}, issueId: "ABC-123"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				w.Write([]byte(``)) //nolint:errcheck
			})
			if err := api.DoTransition(tt.args.issueId, tt.args.transition); (err != nil) != tt.wantErr {
				t.Errorf("DoTransition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_httpJiraApi_FindTransitions(t *testing.T) {
	type args struct {
		issueId string
	}
	tests := []struct {
		name    string
		args    args
		want    []IssueTransition
		wantErr bool
	}{
		{"should find transitions without error",
			args{issueId: "ABC-123"},
			[]IssueTransition{{Id: "11", Name: "To Do"}, {Id: "21", Name: "In Progress"}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body := `
{
    "transitions": [
        {
            "id": "11",
            "name": "To Do"
        },
        {
            "id": "21",
            "name": "In Progress"
        }
    ]
}
`
				w.Write([]byte(body)) //nolint:errcheck
			})
			got, err := api.FindTransitions(tt.args.issueId)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindTransitions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindTransitions() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_httpJiraApi_DoTransition_sendsTransitionObject(t *testing.T) {
	// given
	var method, path, body string
	api := NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(204)
	})

	// when
	err := api.DoTransition("ABC-123", &IssueTransition{Id: "31", Name: "Done"})

	// then - the documented shape, {"transition":{"id":"31"}}
	if err != nil {
		t.Fatalf("DoTransition() error = %v", err)
	}
	if method != "POST" || path != "/rest/api/2/issue/ABC-123/transitions" {
		t.Errorf("DoTransition() sent %s %s", method, path)
	}
	if body != `{"transition":{"id":"31"}}` {
		t.Errorf("DoTransition() body = %s", body)
	}
}

func Test_httpJiraApi_FindTransitions_returnsRequestError(t *testing.T) {
	// given
	api := NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	// when
	got, err := api.FindTransitions("ABC-404")

	// then - the http error, not a deserialization error
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("FindTransitions() error = %v, want a 404 request error", err)
	}
	if got != nil {
		t.Errorf("FindTransitions() got = %v, want nil", got)
	}
}
