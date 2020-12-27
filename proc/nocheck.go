package proc

import (
	"os"
)

// Empty checker, does not do any checking at all, always succeeds.
// Can be used as an example to start the development on a new checker.
type noChecker struct {}

func (d noChecker) prepareCheck(fqn string, fi os.FileInfo) (interface{}, error) {
	return nil, nil
}

func (d noChecker) executeCheck(fqn string, data interface{}, fi os.FileInfo) error {
	return nil
}