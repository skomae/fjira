package issues

import (
	"testing"

	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/ui"
	"github.com/stretchr/testify/assert"
)

func Test_buildSearchIssuesJql(t *testing.T) {
	type args struct {
		project          *jira.Project
		query            string
		status           *jira.IssueStatus
		user             *jira.User
		label            string
		excludedStatuses []*jira.IssueStatus
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"should create valid jql", args{project: &jira.Project{Id: "123"}}, "project=123 ORDER BY status"},
		{"should create valid jql", args{project: &jira.Project{Id: "123"}, query: "abc"}, "project=123 AND summary~\"abc*\" ORDER BY status"},
		{"should create valid jql", args{project: &jira.Project{Id: ui.MessageAll, Key: ui.MessageAll}, query: "abc"}, "summary~\"abc*\" ORDER BY status"},
		{"should create valid jql", args{
			project: &jira.Project{Id: "123"}, query: "abc", status: &jira.IssueStatus{Id: "st1"}},
			"project=123 AND summary~\"abc*\" AND status=st1 ORDER BY status",
		},
		{"should create valid jql", args{
			project: &jira.Project{Id: "123"}, query: "abc", status: &jira.IssueStatus{Id: "st1"}, user: &jira.User{AccountId: "us1"}},
			"project=123 AND summary~\"abc*\" AND status=st1 AND assignee=us1 ORDER BY status",
		},
		{"should create valid jql", args{project: &jira.Project{Id: "123"}, label: "test"}, "project=123 AND labels=test ORDER BY status"},
		{"should create valid jql", args{project: &jira.Project{Id: "123"}, user: &jira.User{Name: "bob"}}, "project=123 AND assignee=bob ORDER BY status"},
		{"should create valid jql with exclude status", args{project: &jira.Project{Id: "123"}, excludedStatuses: []*jira.IssueStatus{{Id: "done"}}}, "project=123 AND status!=done ORDER BY status"},
		{"should create valid jql with multiple excluded statuses", args{project: &jira.Project{Id: "123"}, excludedStatuses: []*jira.IssueStatus{{Id: "done"}, {Id: "closed"}}}, "project=123 AND status!=done AND status!=closed ORDER BY status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, BuildSearchIssuesJql(tt.args.project, tt.args.query, tt.args.status, tt.args.user, tt.args.label, tt.args.excludedStatuses), "BuildSearchIssuesJql(%v, %v, %v, %v, %v, %v)", tt.args.project, tt.args.query, tt.args.status, tt.args.user, tt.args.label, tt.args.excludedStatuses)
		})
	}
}
