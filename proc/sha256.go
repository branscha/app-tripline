package proc

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

type sha256Checker struct {}

func (d sha256Checker) prepareCheck(fqn string, fi os.FileInfo) (interface{}, error) {
	f, err := os.Open(fqn)
	if err != nil {
		return nil, fmt.Errorf("open file")
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("calculate sha256")
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (d sha256Checker) executeCheck(fqn string, data interface{}, fi os.FileInfo) error {
	expectedHash, ok := data.(string)
	if !ok {
		return fmt.Errorf("data corrupt")
	}

	f, err := os.Open(fqn)
	if err != nil {
		return fmt.Errorf("open file")
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("calculate sha256")
	}
	actualHash := fmt.Sprintf("%x", h.Sum(nil))

	if expectedHash != actualHash {
		return fmt.Errorf("expected %s actual %s", expectedHash, actualHash)
	}
	return nil
}