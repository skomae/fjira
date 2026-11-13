package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mk-5/fjira/internal/jira"
	"github.com/stretchr/testify/assert"
)

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
}

// fakeJira is a minimal jira for the write commands. TEST-1 is "To Do",
// assigned to Old Owner; every request is recorded.
type fakeJira struct {
	mu       sync.Mutex
	requests []recordedRequest
}

func (f *fakeJira) writes() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var w []recordedRequest
	for _, r := range f.requests {
		if r.Method != http.MethodGet {
			w = append(w, r)
		}
	}
	return w
}

func writeTestServer(t *testing.T) (*httptest.Server, *fakeJira) {
	t.Helper()
	f := &fakeJira{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body)})
		f.mu.Unlock()
		if r.Method != http.MethodGet {
			w.WriteHeader(204)
			return
		}
		switch r.URL.Path {
		case "/rest/api/2/issue/TEST-1":
			_, _ = w.Write([]byte(`{"key": "TEST-1", "fields": {"summary": "a summary",
				"status": {"name": "To Do", "statusCategory": {"key": "new"}},
				"assignee": {"accountId": "acc-old", "displayName": "Old Owner"}}}`))
		case "/rest/api/2/issue/TEST-1/transitions":
			_, _ = w.Write([]byte(`{"transitions": [
				{"id": "11", "name": "Start Progress", "to": {"id": "3", "name": "In Progress"}},
				{"id": "21", "name": "Done", "to": {"id": "10001", "name": "Done"}},
				{"id": "31", "name": "Deploy", "to": {"id": "10002", "name": "Deployed"}},
				{"id": "41", "name": "Back to backlog", "to": {"id": "1", "name": "To Do"}}
			]}`))
		case "/rest/api/2/user/assignable/search":
			users := map[string]string{
				"alice":       `[{"accountId": "acc-alice", "displayName": "Alice Liddell", "emailAddress": "alice@example.com"}]`,
				"an":          `[{"accountId": "acc-anna", "displayName": "Anna Smith", "emailAddress": "anna@example.com"}, {"accountId": "acc-annabel", "displayName": "Annabel Lee", "emailAddress": "annabel@example.com"}]`,
				"anna smith":  `[{"accountId": "acc-anna", "displayName": "Anna Smith", "emailAddress": "anna@example.com"}, {"accountId": "acc-annabel", "displayName": "Annabel Lee", "emailAddress": "annabel@example.com"}]`,
				"u":           `[{"accountId": "acc-me", "displayName": "Me Myself", "emailAddress": "u@example.com"}]`,
				"old":         `[{"accountId": "acc-old", "displayName": "Old Owner"}]`,
				"nobody-here": `[]`,
			}
			if u, ok := users[strings.ToLower(r.URL.Query().Get("query"))]; ok {
				_, _ = w.Write([]byte(u))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	return server, f
}

func decodeDoc(t *testing.T, out string) map[string]any {
	t.Helper()
	var doc map[string]any
	assert.NoError(t, json.Unmarshal([]byte(out), &doc), out)
	return doc
}

// --- comment

func TestHeadlessCommentCmd(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when
	out, _, err := runHeadlessCmd(t, GetHeadlessCommentCmd(), server.URL, "", "TEST-1", "--body", "hello\nworld", "--json")

	// then
	assert.NoError(t, err)
	assert.Equal(t, []recordedRequest{{Method: "POST", Path: "/rest/api/2/issue/TEST-1/comment", Body: `{"body":"hello\nworld"}`}}, f.writes())
	doc := decodeDoc(t, out)
	assert.Equal(t, "comment", doc["kind"])
	assert.Equal(t, "testws", doc["workspace"])
	assert.Equal(t, "TEST-1", doc["key"])
	assert.Equal(t, false, doc["dryRun"])
	assert.Equal(t, true, doc["changed"])
	assert.Equal(t, "hello\nworld", doc["body"])
	issue := doc["issue"].(map[string]any)
	assert.Equal(t, "issue", issue["kind"])
	assert.Equal(t, "TEST-1", issue["key"])
}

func TestHeadlessCommentCmd_bodyFile(t *testing.T) {
	// given
	server, f := writeTestServer(t)
	path := filepath.Join(t.TempDir(), "c.md")
	assert.NoError(t, os.WriteFile(path, []byte("from a file"), 0o600))

	// when
	_, _, err := runHeadlessCmd(t, GetHeadlessCommentCmd(), server.URL, "", "TEST-1", "--body-file", path, "--json")

	// then
	assert.NoError(t, err)
	assert.Equal(t, `{"body":"from a file"}`, f.writes()[0].Body)
}

func TestHeadlessCommentCmd_bodyFileStdin(t *testing.T) {
	// given
	server, f := writeTestServer(t)
	cmd := GetHeadlessCommentCmd()
	cmd.SetIn(strings.NewReader("from stdin"))

	// when
	_, _, err := runHeadlessCmd(t, cmd, server.URL, "", "TEST-1", "--body-file", "-", "--json")

	// then
	assert.NoError(t, err)
	assert.Equal(t, `{"body":"from stdin"}`, f.writes()[0].Body)
}

func TestHeadlessCommentCmd_dryRun(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when
	out, _, err := runHeadlessCmd(t, GetHeadlessCommentCmd(), server.URL, "", "TEST-1", "--body", "test", "--json", "--dry-run")

	// then
	assert.NoError(t, err)
	assert.Empty(t, f.writes())
	doc := decodeDoc(t, out)
	assert.Equal(t, true, doc["dryRun"])
	assert.Equal(t, true, doc["changed"])
	assert.Equal(t, "test", doc["body"])
	assert.NotContains(t, doc, "issue")
}

func TestHeadlessCommentCmd_errors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"empty body", []string{"TEST-1", "--body", "  \n", "--json"}, "empty"},
		{"no body", []string{"TEST-1", "--json"}, "body"},
		{"both bodies", []string{"TEST-1", "--body", "a", "--body-file", "-", "--json"}, "body"},
		{"missing --json", []string{"TEST-1", "--body", "a"}, "json"},
		{"bad key", []string{"not a key", "--body", "a", "--json"}, "invalid issue key"},
		{"unknown issue", []string{"TEST-404", "--body", "a", "--json"}, "404"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, f := writeTestServer(t)
			_, _, err := runHeadlessCmd(t, GetHeadlessCommentCmd(), server.URL, "", tt.args...)
			assert.ErrorContains(t, err, tt.wantErr)
			assert.Empty(t, f.writes())
		})
	}
}

// --- transition

func TestHeadlessTransitionCmd(t *testing.T) {
	tests := []struct {
		name   string
		to     string
		wantId string
	}{
		{"exact transition name", "start progress", "11"},
		{"exact target status", "In Progress", "11"},
		{"unique prefix", "Dep", "31"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			server, f := writeTestServer(t)

			// when
			out, _, err := runHeadlessCmd(t, GetHeadlessTransitionCmd(), server.URL, "", "TEST-1", "--to", tt.to, "--json")

			// then - the documented body shape, {"transition":{"id":...}}
			assert.NoError(t, err)
			assert.Equal(t, []recordedRequest{{Method: "POST", Path: "/rest/api/2/issue/TEST-1/transitions", Body: `{"transition":{"id":"` + tt.wantId + `"}}`}}, f.writes())
			doc := decodeDoc(t, out)
			assert.Equal(t, "transition", doc["kind"])
			assert.Equal(t, false, doc["dryRun"])
			assert.Equal(t, true, doc["changed"])
			assert.Equal(t, "To Do", doc["from"])
			assert.Equal(t, tt.wantId, doc["transition"].(map[string]any)["id"])
			assert.Equal(t, "TEST-1", doc["issue"].(map[string]any)["key"])
		})
	}
}

func TestHeadlessTransitionCmd_dryRun(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when
	out, _, err := runHeadlessCmd(t, GetHeadlessTransitionCmd(), server.URL, "", "TEST-1", "--to", "done", "--json", "--dry-run")

	// then
	assert.NoError(t, err)
	assert.Empty(t, f.writes())
	doc := decodeDoc(t, out)
	assert.Equal(t, true, doc["dryRun"])
	assert.Equal(t, true, doc["changed"])
	assert.Equal(t, map[string]any{"id": "21", "name": "Done", "to": "Done"}, doc["transition"])
	assert.NotContains(t, doc, "issue")
}

func TestHeadlessTransitionCmd_alreadyInStatusIsNoop(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when - TEST-1 is already "To Do"
	out, _, err := runHeadlessCmd(t, GetHeadlessTransitionCmd(), server.URL, "", "TEST-1", "--to", "to do", "--json")

	// then
	assert.NoError(t, err)
	assert.Empty(t, f.writes())
	doc := decodeDoc(t, out)
	assert.Equal(t, false, doc["changed"])
	assert.Equal(t, false, doc["dryRun"])
	assert.Equal(t, "To Do", doc["from"])
	assert.NotContains(t, doc, "issue")
}

func TestHeadlessTransitionCmd_transitionToCurrentStatusIsNoop(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when - "Back to backlog" leads to "To Do", where TEST-1 already is
	out, _, err := runHeadlessCmd(t, GetHeadlessTransitionCmd(), server.URL, "", "TEST-1", "--to", "back to backlog", "--json")

	// then
	assert.NoError(t, err)
	assert.Empty(t, f.writes())
	doc := decodeDoc(t, out)
	assert.Equal(t, false, doc["changed"])
	assert.Equal(t, "41", doc["transition"].(map[string]any)["id"])
}

func TestHeadlessTransitionCmd_list(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when
	out, _, err := runHeadlessCmd(t, GetHeadlessTransitionCmd(), server.URL, "", "TEST-1", "--list", "--json")

	// then
	assert.NoError(t, err)
	assert.Empty(t, f.writes())
	doc := decodeDoc(t, out)
	assert.Equal(t, "transitions", doc["kind"])
	assert.Equal(t, "TEST-1", doc["key"])
	assert.Equal(t, "To Do", doc["from"])
	transitions := doc["transitions"].([]any)
	assert.Len(t, transitions, 4)
	assert.Equal(t, map[string]any{"id": "11", "name": "Start Progress", "to": "In Progress"}, transitions[0])
}

func TestHeadlessTransitionCmd_errors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr []string
	}{
		{"ambiguous prefix", []string{"TEST-1", "--to", "d", "--json"}, []string{"ambiguous", `"Done" (id 21)`, `"Deploy" (id 31)`}},
		{"no match", []string{"TEST-1", "--to", "Shipped", "--json"}, []string{"no transition matches", `"Start Progress" (id 11) -> "In Progress"`}},
		{"neither --to nor --list", []string{"TEST-1", "--json"}, []string{"list", "to"}},
		{"both --to and --list", []string{"TEST-1", "--to", "Done", "--list", "--json"}, []string{"list", "to"}},
		{"empty --to", []string{"TEST-1", "--to", " ", "--json"}, []string{"empty"}},
		{"missing --json", []string{"TEST-1", "--to", "Done"}, []string{"json"}},
		{"unknown issue", []string{"TEST-404", "--to", "Done", "--json"}, []string{"404"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, f := writeTestServer(t)
			_, _, err := runHeadlessCmd(t, GetHeadlessTransitionCmd(), server.URL, "", tt.args...)
			for _, want := range tt.wantErr {
				assert.ErrorContains(t, err, want)
			}
			assert.Empty(t, f.writes())
		})
	}
}

// --- assign

func TestHeadlessAssignCmd(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when
	out, _, err := runHeadlessCmd(t, GetHeadlessAssignCmd(), server.URL, "", "TEST-1", "--to", "alice", "--json")

	// then - searched within the key's project, assigned by account id (cloud)
	assert.NoError(t, err)
	var search recordedRequest
	for _, r := range f.requests {
		if r.Path == "/rest/api/2/user/assignable/search" {
			search = r
		}
	}
	assert.Contains(t, search.Query, "project=TEST")
	assert.Contains(t, search.Query, "query=alice")
	assert.Equal(t, []recordedRequest{{Method: "PUT", Path: "/rest/api/2/issue/TEST-1/assignee", Body: `{"accountId":"acc-alice"}`}}, f.writes())
	doc := decodeDoc(t, out)
	assert.Equal(t, "assign", doc["kind"])
	assert.Equal(t, false, doc["dryRun"])
	assert.Equal(t, true, doc["changed"])
	assert.Equal(t, map[string]any{"displayName": "Alice Liddell", "email": "alice@example.com", "accountId": "acc-alice"}, doc["assignee"])
	assert.Equal(t, map[string]any{"displayName": "Old Owner", "accountId": "acc-old"}, doc["from"])
	assert.Equal(t, "TEST-1", doc["issue"].(map[string]any)["key"])
}

func TestHeadlessAssignCmd_exactMatchAmongSeveral(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when - "Anna Smith" returns two users, one with that exact display name
	out, _, err := runHeadlessCmd(t, GetHeadlessAssignCmd(), server.URL, "", "TEST-1", "--to", "Anna Smith", "--json", "--dry-run")

	// then
	assert.NoError(t, err)
	assert.Empty(t, f.writes())
	assert.Equal(t, "acc-anna", decodeDoc(t, out)["assignee"].(map[string]any)["accountId"])
}

func TestHeadlessAssignCmd_meDryRun(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when - --me queries with the workspace's jiraUsername ("u")
	out, _, err := runHeadlessCmd(t, GetHeadlessAssignCmd(), server.URL, "", "TEST-1", "--me", "--json", "--dry-run")

	// then
	assert.NoError(t, err)
	assert.Empty(t, f.writes())
	doc := decodeDoc(t, out)
	assert.Equal(t, true, doc["dryRun"])
	assert.Equal(t, true, doc["changed"])
	assert.Equal(t, "acc-me", doc["assignee"].(map[string]any)["accountId"])
	assert.NotContains(t, doc, "issue")
}

func TestHeadlessAssignCmd_alreadyAssignedIsNoop(t *testing.T) {
	// given
	server, f := writeTestServer(t)

	// when
	out, _, err := runHeadlessCmd(t, GetHeadlessAssignCmd(), server.URL, "", "TEST-1", "--to", "old", "--json")

	// then
	assert.NoError(t, err)
	assert.Empty(t, f.writes())
	assert.Equal(t, false, decodeDoc(t, out)["changed"])
}

func TestHeadlessAssignCmd_errors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr []string
	}{
		{"ambiguous user", []string{"TEST-1", "--to", "an", "--json"}, []string{"ambiguous", "Anna Smith <anna@example.com>", "Annabel Lee <annabel@example.com>"}},
		{"no user", []string{"TEST-1", "--to", "nobody-here", "--json"}, []string{"no assignable user"}},
		{"neither --to nor --me", []string{"TEST-1", "--json"}, []string{"me", "to"}},
		{"both --to and --me", []string{"TEST-1", "--to", "alice", "--me", "--json"}, []string{"me", "to"}},
		{"missing --json", []string{"TEST-1", "--to", "alice"}, []string{"json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, f := writeTestServer(t)
			_, _, err := runHeadlessCmd(t, GetHeadlessAssignCmd(), server.URL, "", tt.args...)
			for _, want := range tt.wantErr {
				assert.ErrorContains(t, err, want)
			}
			assert.Empty(t, f.writes())
		})
	}
}

func TestDescribeUsers_capsCandidates(t *testing.T) {
	// given
	users := make([]jira.User, 13)
	for i := range users {
		users[i] = jira.User{DisplayName: fmt.Sprintf("User %d", i)}
	}

	// when
	got := describeUsers(users)

	// then - ten names, then a count of the rest
	assert.Equal(t, maxListedCandidates+1, strings.Count(got, ", ")+1)
	assert.Contains(t, got, "User 9")
	assert.NotContains(t, got, "User 10")
	assert.True(t, strings.HasSuffix(got, "... and 3 more"))
}
