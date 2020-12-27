package proc

import (
	"fmt"
	"os"
)

// Type permissionsChecker verifies if the file permissions have changed since recording them in the database.
type permissionsChecker struct {}

func (d permissionsChecker) prepareCheck(fqn string, fi os.FileInfo) (interface{}, error) {
	// Permissions will be saved as a string "-rw-r--r--"
	return fmt.Sprintf("%s", fi.Mode()), nil
}

func (d permissionsChecker) executeCheck(fqn string, data interface{}, fi os.FileInfo) error {
	// Retrieve the saved permissions string, verify that it it still a string.
	expectedMode, ok := data.(string)
	if !ok {
		return fmt.Errorf("corrupt data, expected string")
	}

	// Get the current permissions and verify them against the stored permissions.
	actualMode := fmt.Sprintf("%s", fi.Mode())
	if expectedMode != actualMode {
		return fmt.Errorf("expected %s actual %s", expectedMode, actualMode)
	}
	return nil
}
