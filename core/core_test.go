package core

import "testing"

// TestCoreVersion is the test function that verifies the core version string is populated correctly.
func TestCoreVersion(t *testing.T) {
	if CoreVersion == "" {
		t.Error("CoreVersion should not be empty")
	}
}
