package helmer

import (
	"fmt"

	"github.com/jenkins-x/jx/v2/pkg/util"
)

// HandleExternalFileRefs recursively scans the element map structure,
// looking for nested maps. If it finds keys that match any key-value pair in possibles it will call the handler.
// The jsonPath is used for referencing the path in the map structure when reporting errors.
func HandleExternalFileRefs(element interface{}, possibles map[string]string, jsonPath string,
	handler func(path string, element map[string]interface{}, key string) error) error {
	if jsonPath == "" {
		// set zero value
		jsonPath = "$"
	}
	if e, ok := element.(map[string]interface{}); ok {
		for k, v := range e {
			if paths, ok := possibles[k]; ok {
				if v == nil || util.IsZeroOfUnderlyingType(v) {
					// There is a filename in the directory structure that matches this key, and it has no value,
					// so we handle it
					err := handler(paths, e, k)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("value at %s must be empty but is %v", jsonPath, v)
				}
			} else {
				// keep on recursing
				jsonPath = fmt.Sprintf("%s.%s", jsonPath, k)
				err := HandleExternalFileRefs(v, possibles, jsonPath, handler)
				if err != nil {
					return err
				}
			}
		}
	}
	// If it's not an object, we can't do much with it
	return nil
}
