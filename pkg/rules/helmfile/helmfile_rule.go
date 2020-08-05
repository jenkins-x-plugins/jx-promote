package helmfile

import (
	"path/filepath"

	"github.com/jenkins-x/jx-helpers/pkg/yaml2s"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/roboll/helmfile/pkg/state"
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

	err := modifyHelmfile(r, filepath.Join(r.Dir, rule.Path), rule.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to modify chart files in dir %s", r.Dir)
	}
	return nil
}

// ModifyAppsFile modifies the 'jx-apps.yml' file to add/update/remove apps
func modifyHelmfile(r *rules.PromoteRule, file string, promoteNs string) error {
	exists, err := util.FileExists(file)
	if err != nil {
		return errors.Wrapf(err, "failed to detect if file exists %s", file)
	}
	if !exists {
		return errors.Errorf("file does not exist %s", file)
	}

	state := &state.HelmState{}
	err = yaml2s.LoadFile(file, state)
	if err != nil {
		return errors.Wrapf(err, "failed to load file %s", file)
	}

	err = modifyHelmfileApps(r, state, promoteNs)
	if err != nil {
		return err
	}

	err = yaml2s.SaveFile(state, file)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", file)
	}
	return nil
}

func modifyHelmfileApps(r *rules.PromoteRule, helmfile *state.HelmState, promoteNs string) error {
	if r.DevEnvContext == nil {
		return errors.Errorf("no devEnvContext")
	}
	app := r.AppName
	version := r.Version
	details, err := r.DevEnvContext.ChartDetails(app, r.HelmRepositoryURL)
	if err != nil {
		return errors.Wrapf(err, "failed to get chart details for %s repo %s", app, r.HelmRepositoryURL)
	}

	for i := range helmfile.Releases {
		appConfig := &helmfile.Releases[i]
		if appConfig.Name == app || appConfig.Name == details.Name {
			appConfig.Version = version
			return nil
		}
	}
	chartName := details.Name
	if details.Prefix == "" {
		// TODO figure out correct prefix!
		details.Prefix = "dev"
	}
	if details.Prefix != "" {
		chartName = details.Prefix + "/" + details.LocalName
	}
	helmfile.Releases = append(helmfile.Releases, state.ReleaseSpec{
		Name:      details.Name,
		Chart:     chartName,
		Version:   version,
		Namespace: promoteNs,
	})
	return nil
}
