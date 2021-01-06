package proc

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

type childChecker struct{}

func (d childChecker) prepareCheck(fqn string, _ os.FileInfo) (interface{}, error) {
	childList, err := childList(fqn)
	return childList, err
}

func (d childChecker) executeCheck(fqn string, data interface{}, _ os.FileInfo) error {
	expectedChildList, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("corrupt child data")
	}

	actualChildList, err := childList(fqn)
	if err != nil {
		return err
	}

	diffResult := make([]string, 0)
	diff := make(map[string]bool)
	for _, expChild := range expectedChildList {
		expectedChildStr, ok := expChild.(string)
		if !ok {
			return fmt.Errorf("corrupt child data")
		}
		diff[expectedChildStr] = true
	}
	for _, actualChild := range actualChildList {
		_, found := diff[actualChild]
		if found {
			// We found it, just remove from the map.
			delete(diff, actualChild)
		} else {
			diffResult = append(diffResult, fmt.Sprintf("new child %q", actualChild))
		}
	}
	for remChild := range diff {
		diffResult = append(diffResult, fmt.Sprintf("removed child %q", remChild))
	}

	if len(diffResult) > 0 {
		return fmt.Errorf(strings.Join(diffResult, ","))
	} else {
		return nil
	}
}

func childList(fqn string) ([]string, error) {
	children, err := ioutil.ReadDir(fqn)
	if err != nil {
		return nil, err
	}

	childList := make([]string, 0)
	for _, child := range children {
		childName := child.Name()
		childList = append(childList, childName)
	}
	return childList, nil
}
