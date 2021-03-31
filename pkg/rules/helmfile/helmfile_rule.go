package helmfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"

	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/yaml2s"
	"github.com/jenkins-x-plugins/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x-plugins/jx-promote/pkg/envctx"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
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

	err := modifyHelmfile(r, rule, filepath.Join(r.Dir, rule.Path), rule.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to modify chart files in dir %s", r.Dir)
	}
	return nil
}

// ModifyAppsFile modifies the 'jx-apps.yml' file to add/update/remove apps
func modifyHelmfile(r *rules.PromoteRule, rule *v1alpha1.HelmfileRule, file string, promoteNs string) error {
	exists, err := files.FileExists(file)
	if err != nil {
		return errors.Wrapf(err, "failed to detect if file exists %s", file)
	}

	st := &state.HelmState{}
	if exists {
		err = yaml2s.LoadFile(file, st)
		if err != nil {
			return errors.Wrapf(err, "failed to load file %s", file)
		}
	}

	dirName, _ := filepath.Split(rule.Path)
	nestedHelmfile := dirName != ""
	err = modifyHelmfileApps(r, st, promoteNs, nestedHelmfile)
	if err != nil {
		return err
	}

	dir := filepath.Dir(file)
	err = os.MkdirAll(dir, files.DefaultDirWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create directory for helmfile %s", dir)
	}

	err = yaml2s.SaveFile(st, file)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", file)
	}

	if !nestedHelmfile {
		return nil
	}

	// lets make sure we reference the nested helmfile in the root helmfile
	rootFile := filepath.Join(r.Dir, "helmfile.yaml")
	rootState := &state.HelmState{}
	err = yaml2s.LoadFile(rootFile, rootState)
	if err != nil {
		return errors.Wrapf(err, "failed to load file %s", rootFile)
	}
	nestedPath := rule.Path
	for _, s := range rootState.Helmfiles {
		matches, err := filepath.Match(s.Path, nestedPath)
		if err == nil && matches {
			return nil
		}
	}
	// lets add the path
	rootState.Helmfiles = append(rootState.Helmfiles, state.SubHelmfileSpec{
		Path: nestedPath,
	})
	err = yaml2s.SaveFile(rootState, rootFile)
	if err != nil {
		return errors.Wrapf(err, "failed to save root helmfile after adding nested helmfile to %s", rootFile)
	}
	return nil
}

func modifyHelmfileApps(r *rules.PromoteRule, helmfile *state.HelmState, promoteNs string, nestedHelmfile bool) error {
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
	defaultPrefix(helmfile, r.DevEnvContext, details, "dev")

	if promoteNs == "" {
		promoteNs = r.Namespace
		if promoteNs == "" {
			promoteNs = "jx"
		}
	}

	isRemoteEnv := r.DevEnvContext.DevEnv.Spec.RemoteCluster

	keepOldReleases := r.Config.Spec.HelmfileRule.KeepOldReleases || contains(r.Config.Spec.HelmfileRule.KeepOldVersions, details.Name)

	if nestedHelmfile {

		if len(helmfile.Releases) == 0 {
			// for nested helmfiles when adding the first release, set it up as the override
			// then when future releases are added they can omit the namespace if their namespace matches this override
			// if different namespaces are required for releases, manual edits should be done to
			// set the namespace of EVERY release and make OverrideNamespace blank
			if promoteNs != "" && helmfile.OverrideNamespace == "" {
				helmfile.OverrideNamespace = promoteNs
			}
		}

		found := false
		if !keepOldReleases {
			for i := range helmfile.Releases {
				release := &helmfile.Releases[i]
				if release.Name == app || release.Name == details.Name {
					release.Version = version
					found = true
					return nil
				}
			}
		}
		if !found {
			ns := ""
			if promoteNs != helmfile.OverrideNamespace {
				ns = promoteNs
			}
			newReleaseName := details.LocalName
			if keepOldReleases {
				newReleaseName = fmt.Sprintf("%s-%s", details.LocalName, strings.Replace(version, ".", "-", -1))
			}
			helmfile.Releases = append(helmfile.Releases, state.ReleaseSpec{
				Name:      newReleaseName,
				Chart:     details.Name,
				Namespace: ns,
				Version:   version,
			})
		}
		return nil
	}
	found := false
	if !keepOldReleases {
		for i := range helmfile.Releases {
			release := &helmfile.Releases[i]
			if (release.Name == app || release.Name == details.Name) && (release.Namespace == promoteNs || isRemoteEnv) {
				release.Version = version
				found = true
				return nil
			}
		}
	}

	if !found {
		newReleaseName := details.LocalName
		if keepOldReleases {
			newReleaseName = fmt.Sprintf("%s-%s", details.LocalName, strings.Replace(version, ".", "-", -1))
		}
		helmfile.Releases = append(helmfile.Releases, state.ReleaseSpec{
			Name:      newReleaseName,
			Chart:     details.Name,
			Version:   version,
			Namespace: promoteNs,
		})
	}

	return nil
}

// defaultPrefix lets find a chart prefix / repository name for the URL that does not clash with
// any other existing repositories in the helmfile
func defaultPrefix(appsConfig *state.HelmState, envctx *envctx.EnvironmentContext, d *envctx.ChartDetails, defaultPrefix string) {
	if d.Prefix != "" {
		return
	}
	found := false
	oci := false
	if envctx.Requirements != nil {
		oci = envctx.Requirements.Cluster.ChartKind == jxcore.ChartRepositoryTypeOCI
	}
	prefixes := map[string]string{}
	urls := map[string]string{}
	for _, r := range appsConfig.Repositories {
		if r.URL == d.Repository {
			found = true
			r.OCI = oci
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
			OCI:  oci,
		})

	}
	d.SetPrefix(prefix)
}

func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}
