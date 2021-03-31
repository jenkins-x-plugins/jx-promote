package promoteconfig

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x-plugins/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// Discover discovers the promote configuration.
//
// if an explicit configuration is found (in a current or parent directory of '.jx/promote.yaml' then that is used.
// otherwise the env/Chart.yaml or 'jx-apps.yaml' are detected
func Discover(dir, promoteNamespace string) (*v1alpha1.Promote, string, error) {
	config, fileName, err := LoadPromote(dir, false)
	if err != nil {
		return config, fileName, errors.Wrapf(err, "failed to load Promote configuration from %s", dir)
	}
	if config != nil {
		return config, fileName, nil
	}

	envChart := filepath.Join(dir, "env", "Chart.yaml")
	exists, err := files.FileExists(envChart)
	if err != nil {
		return nil, "", errors.Wrapf(err, "failed to check if file exists %s", envChart)
	}
	if exists {
		config := v1alpha1.Promote{
			ObjectMeta: metav1.ObjectMeta{
				Name: "generated",
			},
			Spec: v1alpha1.PromoteSpec{
				HelmRule: &v1alpha1.HelmRule{
					Path: "env",
				},
			},
		}
		return &config, "", nil
	}

	path, err := findHelmfile(dir, promoteNamespace)
	if err != nil {
		return nil, "", errors.Wrapf(err, "failed to find helmfile")
	}
	config = &v1alpha1.Promote{
		ObjectMeta: metav1.ObjectMeta{
			Name: "generated",
		},
		Spec: v1alpha1.PromoteSpec{
			HelmfileRule: &v1alpha1.HelmfileRule{
				Path:      path,
				Namespace: promoteNamespace,
			},
		},
	}
	return config, "", nil
}

func findHelmfile(dir, promoteNamespace string) (string, error) {
	helmfilesDir := filepath.Join(dir, "helmfiles")
	exists, err := files.DirExists(helmfilesDir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to detect if dir exists %s", helmfilesDir)
	}
	if !exists || promoteNamespace == "" {
		return "helmfile.yaml", nil
	}
	// lets assume we are using a nested helmfile
	return filepath.Join("helmfiles", promoteNamespace, "helmfile.yaml"), nil
}

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

		exists, err := files.FileExists(fileName)
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
