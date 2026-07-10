package issues

import (
	"os"
	"testing"

	os2 "github.com/mk-5/fjira/internal/os"
)

// TestMain isolates the fjira home dir for the whole issues test binary so that
// filter-persistence writes (saveFilters) land in a throwaway temp dir instead
// of the developer's real ~/.fjira/fjira.yaml. Without this, exercising the
// select-status/assignee/label paths would pollute real config and fail in a
// home-write-restricted CI sandbox.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "fjira-issues-test-home")
	if err != nil {
		panic(err)
	}
	if err := os2.SetUserHomeDir(tmp); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}
