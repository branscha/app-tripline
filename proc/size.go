package proc

import (
	"fmt"
	"os"
	"strconv"
)

type fileSizeChecker struct {}

func (d fileSizeChecker) prepareCheck(fqn string, fi os.FileInfo) (interface{}, error) {
	// Get the file size.
	fileSize := fi.Size()
	// Convert it to a string to preserve int64 precision.
	return strconv.FormatInt(fileSize, 10), nil
}

func (d fileSizeChecker) executeCheck(fqn string, data interface{}, fi os.FileInfo) error {
	// Get the actual file size.
	actualSize := fi.Size()
	// Get the recorded size from a string.
	recordedSizeRepr, ok := data.(string)
	if !ok {
		// The data is not a string...
		return fmt.Errorf("size was not recorded")
	}
	recordedSize, err := strconv.ParseInt(recordedSizeRepr, 10, 64)
	if err!= nil {
		// The string cannot be parsed into an int ...
		return fmt.Errorf("size was not recorded")
	}
	if actualSize != recordedSize {
		// The actual and recorded size differ ...
		return fmt.Errorf("expected %v actual %v", recordedSize, actualSize)
	}
	return nil
}