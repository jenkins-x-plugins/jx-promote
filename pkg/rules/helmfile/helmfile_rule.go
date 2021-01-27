package helmfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/yaml2s"
	"github.com/jenkins-x/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x/jx-promote/pkg/envctx"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/pkg/errors"
	"github.com/roboll/helmfile/pkg/state"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyaml "sigs.k8s.io/yaml"
)

// Rule uses a helmfile.yaml file
func Rule(r *rules.PromoteRule) error {
	config := r.Config
	if config.Spec.HelmfileRule == nil {
		return errors.Errorf("no helmfileRule configured")
	}
	rule := config.Spec.HelmfileRule
	if rule.Path == "" {
		rule.Path = "helmfile.yaml"
	}

	envHelmfileRulePath := filepath.Join(r.Dir, "helmfiles", rule.Namespace, "promote.yaml")
	exists, err := files.FileExists(envHelmfileRulePath)
	if err != nil {
		return errors.Wrapf(err, "failed to check if file exists %s", envHelmfileRulePath)
	}
	if exists {
		// environment specific HelmfileRule
		envHemlfileRule, err := LoadHelmfilePromote(envHelmfileRulePath)
		if err != nil {
			return errors.Wrapf(err, "failed load %s", envHelmfileRulePath)
		}
		config.Spec.HelmfileRule.KeepOldVersions = envHemlfileRule.Spec.KeepOldVersions
	}

	err = modifyHelmfile(r, rule, filepath.Join(r.Dir, rule.Path), rule.Namespace)
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
	defaultPrefix(helmfile, details, "dev")

	if promoteNs == "" {
		promoteNs = r.Namespace
		if promoteNs == "" {
			promoteNs = "jx"
		}
	}

	isRemoteEnv := r.DevEnvContext.DevEnv.Spec.RemoteCluster

	keepOldVersions := contains(r.Config.Spec.HelmfileRule.KeepOldVersions, details.Name)

	if nestedHelmfile {
		// for nested helmfiles we assume we don't need to specify a namespace on each chart
		// as all the charts will use the same namespace
		if promoteNs != "" && helmfile.OverrideNamespace == "" {
			helmfile.OverrideNamespace = promoteNs
		}

		found := false
		if !keepOldVersions {
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
			if keepOldVersions {
				newReleaseName = fmt.Sprintf("%s-%s", details.LocalName, version)
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
	if !keepOldVersions {
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
		if keepOldVersions {
			newReleaseName = fmt.Sprintf("%s-%s", details.LocalName, version)
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

func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

// HelmfilePromote is for configuring promotion for an environment
type HelmfilePromote struct {
	metav1.TypeMeta `json:",inline"`

	Spec HelmfilePromoteSpec `json:"spec"`
}

// HelmfilePromoteSpec defines the configuration for an environment
type HelmfilePromoteSpec struct {
	// keepOldVersions if specified is a list of release names and if the release name is in this list then the old versions are kept
	KeepOldVersions []string `json:"keepOldVersions,omitempty"`
}

// LoadHelmfilePromote loads a HelmfilePromote from a specific YAML file
func LoadHelmfilePromote(fileName string) (*HelmfilePromote, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file %s", fileName)
	}
	config := &HelmfilePromote{}
	err = kyaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML file %s due to %s", fileName, err)
	}
	return config, nil
}
