package helmfile

import (
	"io/ioutil"
	"path/filepath"

	"github.com/jenkins-x/jx-promote/pkg/helmfile"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// HelmfileRule uses a jx-apps.yml file
func HelmfileRule(r *rules.PromoteRule) error {
	config := r.Config
	if config.Spec.HelmfileRule == nil {
		return errors.Errorf("no helmfileRule configured")
	}
	rule := config.Spec.HelmfileRule
	if rule.Path == "" {
		rule.Path = "helmfile.yaml"
	}

	err := modifyHelmfile(r, filepath.Join(r.Dir, rule.Path))
	if err != nil {
		return errors.Wrapf(err, "failed to modify chart files in dir %s", r.Dir)
	}
	return nil
}

// ModifyAppsFile modifies the 'jx-apps.yml' file to add/update/remove apps
func modifyHelmfile(r *rules.PromoteRule, file string) error {
	exists, err := util.FileExists(file)
	if err != nil {
		return errors.Wrapf(err, "failed to detect if file exists %s", file)
	}
	if !exists {
		return errors.Errorf("file does not exist %s", file)
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return errors.Wrapf(err, "failed to load file %s", file)
	}

	state := &helmfile.HelmState{}

	err = yaml.Unmarshal(data, state)
	if err != nil {
		return errors.Wrapf(err, "failed parse YAML file %s", file)
	}

	err = modifyHelmfileApps(r, state)
	if err != nil {
		return err
	}

	data, err = yaml.Marshal(state)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal helmfile state %#v", state)
	}
	err = ioutil.WriteFile(file, data, util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", file)
	}
	return nil
}

func modifyHelmfileApps(r *rules.PromoteRule, state *helmfile.HelmState) error {
	if r.DevEnvContext == nil {
		return errors.Errorf("no devEnvContext")
	}
	app := r.AppName
	version := r.Version
	details, err := r.DevEnvContext.ChartDetails(app, r.HelmRepositoryURL)
	if err != nil {
		return errors.Wrapf(err, "failed to get chart details for %s repo %s", app, r.HelmRepositoryURL)
	}

	for i := range state.Releases {
		appConfig := &state.Releases[i]
		if appConfig.Name == app || appConfig.Name == details.Name {
			appConfig.Version = version
			return nil
		}
	}
	state.Releases = append(state.Releases, helmfile.ReleaseSpec{
		Name:    details.Name,
		Version: version,
	})
	return nil
}
