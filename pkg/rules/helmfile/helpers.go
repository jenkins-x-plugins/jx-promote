package helmfile

import (
	"fmt"

	"github.com/helmfile/helmfile/pkg/state"
	"github.com/jenkins-x/jx-helpers/v3/pkg/yaml2s"
)

// LoadHelmfile loads helmfile from a path
func LoadHelmfile(file string) (*state.HelmState, error) {
	state := &state.HelmState{}
	err := yaml2s.LoadFile(file, state)
	if err != nil {
		return nil, fmt.Errorf("failed to load file %s: %w", file, err)
	}
	return state, nil
}
