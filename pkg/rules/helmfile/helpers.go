package helmfile

import (
	"github.com/jenkins-x/jx-helpers/v3/pkg/yaml2s"
	"github.com/pkg/errors"
	"github.com/roboll/helmfile/pkg/state"
)

// LoadHelmfile loads helmfile from a path
func LoadHelmfile(file string) (*state.HelmState, error) {
	state := &state.HelmState{}
	err := yaml2s.LoadFile(file, state)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load file %s", file)
	}
	return state, nil
}
