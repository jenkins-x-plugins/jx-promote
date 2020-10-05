package helmfile

import (
	"fmt"
	"path/filepath"

	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/yaml2s"
	"github.com/jenkins-x/jx-promote/pkg/envctx"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/pkg/errors"
	"github.com/roboll/helmfile/pkg/state"
)

// HelmfileRule uses a jx-apps.yml file
func Rule(r *rules.PromoteRule) error {
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
	exists, err := files.FileExists(file)
	if err != nil {
		return errors.Wrapf(err, "failed to detect if file exists %s", file)
	}
	if !exists {
		return errors.Errorf("file does not exist %s", file)
	}

	st := &state.HelmState{}
	err = yaml2s.LoadFile(file, st)
	if err != nil {
		return errors.Wrapf(err, "failed to load file %s", file)
	}

	err = modifyHelmfileApps(r, st, promoteNs)
	if err != nil {
		return err
	}

	err = yaml2s.SaveFile(st, file)
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
	if r.HelmRepositoryURL == "" {
		r.HelmRepositoryURL = "http://jenkins-x-chartmuseum:8080"
	}
	details, err := r.DevEnvContext.ChartDetails(app, r.HelmRepositoryURL)
	if err != nil {
		return errors.Wrapf(err, "failed to get chart details for %s repo %s", app, r.HelmRepositoryURL)
	}
	defaultPrefix(helmfile, details, "dev")

	if promoteNs == "" {
		promoteNs = r.Namespace
		if promoteNs == "" {
			promoteNs = "jx"
		}
	}

	isRemoteEnv := r.DevEnvContext.DevEnv.Spec.RemoteCluster

	found := false
	for i := range helmfile.Releases {
		release := &helmfile.Releases[i]
		if (release.Name == app || release.Name == details.Name) && (release.Namespace == promoteNs || isRemoteEnv) {
			release.Version = version
			found = true
			return nil
		}
	}

	if !found {
		helmfile.Releases = append(helmfile.Releases, state.ReleaseSpec{
			Name:      details.LocalName,
			Chart:     details.Name,
			Version:   version,
			Namespace: promoteNs,
		})
	}

	return nil
}

// defaultPrefix lets find a chart prefix / repository name for the URL that does not clash with
// any other existing repositories in the helmfile
func defaultPrefix(appsConfig *state.HelmState, d *envctx.ChartDetails, defaultPrefix string) {
	if d.Prefix != "" {
		return
	}
	found := false
	prefixes := map[string]string{}
	urls := map[string]string{}
	for _, r := range appsConfig.Repositories {
		if r.URL == d.Repository {
			found = true
		}
		if r.Name != "" {
			urls[r.URL] = r.Name
			prefixes[r.Name] = r.URL
		}
	}

	prefix := urls[d.Repository]
	if prefix == "" {
		if prefixes[defaultPrefix] == "" {
			prefix = defaultPrefix
		} else {
			// the defaultPrefix exists and maps to another URL
			// so lets create another similar prefix name as an alias for this repo URL
			i := 2
			for {
				prefix = fmt.Sprintf("%s%d", defaultPrefix, i)
				if prefixes[prefix] == "" {
					break
				}
				i++
			}
		}
	}
	if !found {
		appsConfig.Repositories = append(appsConfig.Repositories, state.RepositorySpec{
			Name: prefix,
			URL:  d.Repository,
		})

	}
	d.SetPrefix(prefix)
}
