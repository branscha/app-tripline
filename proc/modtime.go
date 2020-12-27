package proc

import (
	"fmt"
	"os"
	"time"
)

const storageFormat = time.RFC3339Nano
const displayFormat = time.RFC3339

type modTimeChecker struct {}

func (d modTimeChecker) prepareCheck(fqn string, fi os.FileInfo) (interface{}, error) {
	// Get the file modification time
	mtime := fi.ModTime()
	// Convert it to a string to preserve nano sec precision.
	return mtime.Format(storageFormat), nil
}

func (d modTimeChecker) executeCheck(fqn string, data interface{}, fi os.FileInfo) error {
	// Get the actual modification time
	actualModTime := fi.ModTime()
	actualModTimeRepr := actualModTime.Format(storageFormat)
	// Get the recorded modification time from a string.
	recordedModTimeRepr, ok := data.(string)
	if !ok {
		// The data is not a string...
		return fmt.Errorf("modtime not recorded")
	}
	// We only convert the string to a timestamp to verify that it is correct (and no tampering)
	// We will continue using the string representation though.
	recordedModTime, err := time.Parse(storageFormat, recordedModTimeRepr)
	if err!= nil {
		// The string cannot be parsed into an int ...
		return fmt.Errorf("modtime not recorded")
	}
	// We compare the string representations so we are sure we have the same precision (millis)
	if actualModTimeRepr != recordedModTimeRepr {
		// The actual and recorded modtime differ ...
		// We print out the dates in a more compact format in order not to clutter the output
		return fmt.Errorf("expected '%v' actual '%v'", recordedModTime.Format(displayFormat), actualModTime.Format(displayFormat))
	}
	return nil
}