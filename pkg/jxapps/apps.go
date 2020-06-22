package jxapps

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx-promote/pkg/helmfile"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

const (
	// AppConfigFileName is the name of the applications configuration file
	AppConfigFileName = "jx-apps.yml"
	// PhaseSystem is installed before the apps phase
	PhaseSystem Phase = "system"
	// PhaseApps is installed after the system phase
	PhaseApps Phase = "apps"
)

// PhaseValues the string values for Phases
var PhaseValues = []string{"system", "apps"}

// AppConfig contains the apps to install during boot for helmfile / helm 3
type AppConfig struct {
	// Apps of applications
	Apps []App `json:"apps"`
	// Repositories list of helm repositories
	Repositories []helmfile.RepositorySpec `json:"repositories,omitempty"`
	// DefaultNamespace the default namespace to install applications into
	DefaultNamespace string `json:"defaultNamespace,omitempty"`
}

// App is the configuration of an app used during boot for helmfile / helm 3
type App struct {
	// Name of the application / helm chart
	Name string `json:"name,omitempty"`
	// Repository the helm repository
	Repository string `json:"repository,omitempty"`
	// Namespace to install the application into
	Namespace string `json:"namespace,omitempty"`
	// Phase of the pipeline to install application
	Phase Phase `json:"phase,omitempty"`
	// Version the version to install if you want to override the version from the Version Stream.
	// Note we recommend using the version stream for app versions
	Version string `json:"version,omitempty"`
	// Description an optional description of the app
	Description string `json:"description,omitempty"`
	// Alias an optional alias of the app
	Alias string `json:"alias,omitempty"`
	// Values any explicit value files to be used
	Values []string `json:"values,omitempty"`
	// Hooks is a list of extension points paired with operations, that are executed in specific points of the lifecycle of releases defined in helmfile
	Hooks []helmfile.Hook `json:"hooks,omitempty"`
	// Wait, if set to true, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful
	Wait *bool `json:"wait,omitempty"`
	// Timeout is the time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks, and waits on pod/pvc/svc/deployment readiness) (default 300)
	Timeout *int `json:"timeout,omitempty"`
	// RecreatePods, when set to true, instruct helmfile to perform pods restart for the resource if applicable
	RecreatePods *bool `json:"recreatePods,omitempty"`
	// Force, when set to true, forces resource update through delete/recreate if needed
	Force *bool `json:"force,omitempty"`
	// Installed, when set to true, `delete --purge` the release
	Installed *bool `json:"installed,omitempty"`
	// Atomic, when set to true, restore previous state in case of a failed install/upgrade attempt
	Atomic *bool `json:"atomic,omitempty"`
	// CleanupOnFail, when set to true, the --cleanup-on-fail helm flag is passed to the upgrade command
	CleanupOnFail *bool `json:"cleanupOnFail,omitempty"`
}

// Phase of the pipeline to install application
type Phase string

// LoadAppConfig loads the boot applications configuration file
// if there is not a file called `jx-apps.yml` in the given dir we will scan up the parent
// directories looking for the requirements file as we often run 'jx' steps in sub directories.
func LoadAppConfig(dir string) (*AppConfig, string, error) {
	fileName := AppConfigFileName
	if dir != "" {
		fileName = filepath.Join(dir, fileName)
	}

	exists, err := util.FileExists(fileName)
	if err != nil {
		return nil, fileName, errors.Errorf("error looking up %s in directory %s", fileName, dir)
	}

	config := &AppConfig{}
	if !exists {
		return config, "", nil
	}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return config, fileName, fmt.Errorf("Failed to load file %s due to %s", fileName, err)
	}
	validationErrors, err := util.ValidateYaml(config, data)
	if err != nil {
		return config, fileName, fmt.Errorf("failed to validate YAML file %s due to %s", fileName, err)
	}
	if len(validationErrors) > 0 {
		return config, fileName, fmt.Errorf("Validation failures in YAML file %s:\n%s", fileName, strings.Join(validationErrors, "\n"))
	}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return config, fileName, fmt.Errorf("Failed to unmarshal YAML file %s due to %s", fileName, err)
	}

	// validate all phases are known types, default to apps if not specified
	for _, app := range config.Apps {
		if app.Phase != "" {
			if app.Phase != PhaseSystem && app.Phase != PhaseApps {
				return config, fileName, fmt.Errorf("failed to validate YAML file, invalid phase '%s', needed on of %v",
					string(app.Phase), PhaseValues)
			}
		}
	}

	return config, fileName, err
}

// SaveConfig saves the configuration file to the given project directory
func (c *AppConfig) SaveConfig(fileName string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", fileName)
	}
	return nil
}
