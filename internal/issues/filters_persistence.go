package issues

import (
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/ui"
	"github.com/mk-5/fjira/internal/workspaces"
)

// saveFilters persists the current issue-navigator filter globals for the given
// project. Called after every filter mutation so disk always mirrors the live
// globals. Persistence is best-effort: a save failure must never interrupt the
// navigator, so the error is intentionally ignored.
func saveFilters(project *jira.Project) {
	if project == nil || project.Id == "" || project.Id == ui.MessageAll {
		return
	}
	f := workspaces.ProjectIssueFilters{}
	if searchForStatus != nil && searchForStatus.Name != ui.MessageAll {
		f.StatusId = searchForStatus.Id
		f.StatusName = searchForStatus.Name
	}
	if searchForUser != nil && searchForUser.DisplayName != ui.MessageAll {
		f.AssigneeAccountId = searchForUser.AccountId
		f.AssigneeName = searchForUser.Name
		f.AssigneeDisplay = searchForUser.DisplayName
	}
	if searchForLabel != "" && searchForLabel != ui.MessageAll {
		f.Label = searchForLabel
	}
	for _, es := range excludedStatuses {
		if es != nil && es.Name != ui.MessageAll {
			f.ExcludedStatusIds = append(f.ExcludedStatusIds, es.Id)
			f.ExcludedStatusNames = append(f.ExcludedStatusNames, es.Name)
		}
	}
	f.SortByUpdated = sortByUpdated
	_ = workspaces.SaveIssueFilters(project.Id, f)
}

// restoreFilters loads the saved filters for the given project into the
// navigator globals. It is load-or-clear: when a project has no saved filters
// the globals are reset to nil, so a previous project's filters never bleed
// into a project where those ids don't exist. Reconstructs the jira structs
// from persisted primitives with no API round-trip.
func restoreFilters(project *jira.Project) {
	if project == nil || project.Id == "" || project.Id == ui.MessageAll {
		return
	}
	f, found, err := workspaces.LoadIssueFilters(project.Id)
	// On error, leave the current globals untouched rather than silently
	// wiping a filter the user just set — a read failure shouldn't mutate state.
	if err != nil {
		return
	}
	if !found {
		searchForStatus = nil
		searchForUser = nil
		searchForLabel = ""
		excludedStatuses = nil
		sortByUpdated = false
		return
	}
	if f.StatusId != "" {
		searchForStatus = &jira.IssueStatus{Id: f.StatusId, Name: f.StatusName}
	} else {
		searchForStatus = nil
	}
	if f.AssigneeAccountId != "" || f.AssigneeName != "" {
		searchForUser = &jira.User{
			AccountId:   f.AssigneeAccountId,
			Name:        f.AssigneeName,
			DisplayName: f.AssigneeDisplay,
		}
	} else {
		searchForUser = nil
	}
	searchForLabel = f.Label
	excludedStatuses = nil
	for i, id := range f.ExcludedStatusIds {
		name := ""
		if i < len(f.ExcludedStatusNames) {
			name = f.ExcludedStatusNames[i]
		}
		excludedStatuses = append(excludedStatuses, &jira.IssueStatus{Id: id, Name: name})
	}
	// Unconditional assignment: sortByUpdated is a process-global shared across
	// projects, so it must be set to the saved value (not OR'd in), or one
	// project's sort mode would leak into another. Same load-or-clear invariant
	// as the filters above.
	sortByUpdated = f.SortByUpdated
}
