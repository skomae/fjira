package workspaces

import "fmt"

// IssueFiltersKey builds the storage key for a project's remembered issue
// filters, scoped per connection (workspace) and project.
func IssueFiltersKey(workspace string, projectId string) string {
	return fmt.Sprintf("%s/%s", workspace, projectId)
}

// SaveIssueFilters persists the issue-navigator filters for the current
// workspace and the given project. Errors are returned so callers can decide
// whether to surface them; a failed save must never crash the navigator.
func SaveIssueFilters(projectId string, filters ProjectIssueFilters) error {
	workspace, err := GetCurrent()
	if err != nil {
		return err
	}
	s := NewUserHomeSettingsStorage().(*userHomeSettingsStorage)
	return s.WriteIssueFilters(IssueFiltersKey(workspace, projectId), filters)
}

// LoadIssueFilters returns the saved filters for the current workspace and the
// given project. found=false means this project has no saved filters, so the
// caller should clear all filters rather than leave a previous project's in place.
func LoadIssueFilters(projectId string) (ProjectIssueFilters, bool, error) {
	workspace, err := GetCurrent()
	if err != nil {
		return ProjectIssueFilters{}, false, err
	}
	s := NewUserHomeSettingsStorage().(*userHomeSettingsStorage)
	return s.ReadIssueFilters(IssueFiltersKey(workspace, projectId))
}
