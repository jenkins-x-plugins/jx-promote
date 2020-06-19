package promoteconfig

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/jenkins-x/jx-promote/pkg/apis/boot/v1alpha1"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// LoadPromote loads the boot config from the given directory
func LoadPromote(dir string, failIfMissing bool) (*v1alpha1.Promote, string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", errors.Wrap(err, "creating absolute path")
	}
	relPath := filepath.Join(".jx", "promote.yaml")

	for absolute != "" && absolute != "." && absolute != "/" {
		fileName := filepath.Join(absolute, relPath)
		absolute = filepath.Dir(absolute)

		exists, err := util.FileExists(fileName)
		if err != nil {
			return nil, "", err
		}

		if !exists {
			continue
		}

		config, err := LoadPromoteFile(fileName)
		return config, fileName, err
	}
	if failIfMissing {
		return nil, "", errors.Errorf("%s file not found", relPath)
	}
	return nil, "", nil
}

// LoadPromoteFile loads a specific boot config YAML file
func LoadPromoteFile(fileName string) (*v1alpha1.Promote, error) {
	config := &v1alpha1.Promote{}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load file %s due to %s", fileName, err)
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML file %s due to %s", fileName, err)
	}

	return config, nil
}
