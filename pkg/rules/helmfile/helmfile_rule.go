package helmfile

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jenkins-x-plugins/jx-gitops/pkg/helmfiles"

	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"

	"github.com/helmfile/helmfile/pkg/state"
	"github.com/jenkins-x-plugins/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x-plugins/jx-promote/pkg/envctx"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
)

// HelmfileRule uses a jx-apps.yml file
func Rule(r *rules.PromoteRule) error {
	config := r.Config
	if config.Spec.HelmfileRule == nil {
		return fmt.Errorf("no helmfileRule configured")
	}
	rule := config.Spec.HelmfileRule
	if rule.Path == "" {
		rule.Path = "helmfile.yaml"
	}

	err := modifyHelmfile(r, rule, filepath.Join(r.Dir, rule.Path), rule.Namespace)
	if err != nil {
		return fmt.Errorf("failed to modify chart files in dir %s: %w", r.Dir, err)
	}
	return nil
}

// ModifyAppsFile modifies the 'jx-apps.yml' file to add/update/remove apps
func modifyHelmfile(r *rules.PromoteRule, rule *v1alpha1.HelmfileRule, file, promoteNs string) error {
	exists, err := files.FileExists(file)
	if err != nil {
		return fmt.Errorf("failed to detect if file exists %s: %w", file, err)
	}

	helmstates := []*state.HelmState{{}}
	if exists {
		helmstates, err = helmfiles.LoadHelmfile(file)
		if err != nil {
			return fmt.Errorf("failed to load file %s: %w", file, err)
		}
	}

	dirName, _ := filepath.Split(rule.Path)
	nestedHelmfile := dirName != ""
	err = modifyHelmfileApps(r, helmstates, promoteNs, nestedHelmfile)
	if err != nil {
		return err
	}

	err = helmfiles.SaveHelmfile(file, helmstates)
	if err != nil {
		return fmt.Errorf("failed to save file %s: %w", file, err)
	}

	if !nestedHelmfile {
		return nil
	}

	// lets make sure we reference the nested helmfile in the root helmfile
	rootFile := filepath.Join(r.Dir, "helmfile.yaml")
	rootStates, err := helmfiles.LoadHelmfile(rootFile)
	if err != nil {
		return fmt.Errorf("failed to load file %s: %w", rootFile, err)
	}
	nestedPath := rule.Path
	for _, rootState := range rootStates {
		for _, s := range rootState.Helmfiles {
			matches, err := filepath.Match(s.Path, nestedPath)
			if err == nil && matches {
				return nil
			}
		}
	}
	// lets add the path
	lastRootState := rootStates[len(rootStates)-1]
	lastRootState.Helmfiles = append(lastRootState.Helmfiles, state.SubHelmfileSpec{
		Path: nestedPath,
	})
	err = helmfiles.SaveHelmfile(rootFile, rootStates)
	if err != nil {
		return fmt.Errorf("failed to save root helmfile after adding nested helmfile to %s: %w", rootFile, err)
	}
	return nil
}

func modifyHelmfileApps(r *rules.PromoteRule, helmStates []*state.HelmState, promoteNs string, nestedHelmfile bool) error {
	if r.DevEnvContext == nil {
		return fmt.Errorf("no devEnvContext")
	}
	app := r.AppName
	version := r.Version
	if r.HelmRepositoryURL == "" {
		r.HelmRepositoryURL = "http://jenkins-x-chartmuseum:8080"
	}
	details, err := r.DevEnvContext.ChartDetails(app, r.HelmRepositoryURL)
	if err != nil {
		return fmt.Errorf("failed to get chart details for %s repo %s: %w", app, r.HelmRepositoryURL, err)
	}
	defaultPrefix(helmStates, r.DevEnvContext, details, "dev")

	if promoteNs == "" {
		promoteNs = r.Namespace
		if promoteNs == "" {
			promoteNs = "jx"
		}
	}

	isRemoteEnv := r.DevEnvContext.DevEnv.Spec.RemoteCluster

	keepOldReleases := r.Config.Spec.HelmfileRule.KeepOldReleases || contains(r.Config.Spec.HelmfileRule.KeepOldVersions, details.Name)

	if nestedHelmfile {
		// This is edge case so moved to a separate function
		promoteNestedHelmfileReleases(r, details, promoteNs, helmStates, keepOldReleases)
		return nil
	}

	// Time to use scoring instead of just a simple found.
	highestScorer := new(state.ReleaseSpec)
	highestScore := 0

	if !keepOldReleases {
		for _, helmfile := range helmStates {
			for i := range helmfile.Releases {
				score := 0
				release := &helmfile.Releases[i]

				if release.Name == r.AppName && (release.Namespace == promoteNs || isRemoteEnv) {
					score++
				}

				if (r.ReleaseName != "" && release.Name == r.ReleaseName) && (release.Namespace == promoteNs || isRemoteEnv) {
					// This scores higher as it's a direct match
					score += 2
				}

				if score > highestScore {
					highestScorer = release
					highestScore = score
				}
			}
		}
	}

	if highestScore > 0 {
		highestScorer.Version = r.Version
	} else {
		newReleaseName := details.LocalName
		if r.ReleaseName != "" {
			newReleaseName = r.ReleaseName
		}
		if keepOldReleases {
			newReleaseName = fmt.Sprintf("%s-%s", newReleaseName, strings.ReplaceAll(version, ".", "-"))
		}
		lastHelmState := helmStates[len(helmStates)-1]
		lastHelmState.Releases = append(lastHelmState.Releases, state.ReleaseSpec{
			Name:      newReleaseName,
			Chart:     details.Name,
			Version:   version,
			Namespace: promoteNs,
		})
	}

	return nil
}

func promoteNestedHelmfileReleases(r *rules.PromoteRule, details *envctx.ChartDetails, promoteNs string, helmStates []*state.HelmState, keepOldReleases bool) {

	noReleases := false
	for _, helmfile := range helmStates {
		noReleases = noReleases || len(helmfile.Releases) == 0
	}
	lastHelmState := helmStates[len(helmStates)-1]
	if noReleases {
		// for nested helmfiles when adding the first release, set it up as the override
		// then when future releases are added they can omit the namespace if their namespace matches this override
		// if different namespaces are required for releases, manual edits should be done to
		// set the namespace of EVERY release and make OverrideNamespace blank
		if promoteNs != "" && lastHelmState.OverrideNamespace == "" {
			lastHelmState.OverrideNamespace = promoteNs
		}
	}

	// Time to use scoring instead of just a simple found.
	highestScorer := new(state.ReleaseSpec)
	highestScore := 0
	if !keepOldReleases {
		for _, helmfile := range helmStates {
			for i := range helmfile.Releases {
				score := 0
				release := &helmfile.Releases[i]

				if release.Name == r.AppName {
					score++
				}

				if r.ReleaseName != "" && release.Name == r.ReleaseName {
					score += 2
				}

				if score > highestScore {
					highestScorer = release
					highestScore = score
				}
			}
		}
	}

	if highestScore > 0 {
		highestScorer.Version = r.Version
	} else {
		ns := ""
		if promoteNs != lastHelmState.OverrideNamespace {
			ns = promoteNs
		}
		newReleaseName := details.LocalName
		if r.ReleaseName != "" {
			newReleaseName = r.ReleaseName
		}
		if keepOldReleases {
			newReleaseName = fmt.Sprintf("%s-%s", newReleaseName, strings.ReplaceAll(r.Version, ".", "-"))
		}
		lastHelmState.Releases = append(lastHelmState.Releases, state.ReleaseSpec{
			Name:      newReleaseName,
			Chart:     details.Name,
			Namespace: ns,
			Version:   r.Version,
		})
	}
}

// defaultPrefix lets find a chart prefix / repository name for the URL that does not clash with
// any other existing repositories in the helmfile
func defaultPrefix(helmStates []*state.HelmState, envctx *envctx.EnvironmentContext, d *envctx.ChartDetails, defaultPrefix string) {
	if d.Prefix != "" {
		return
	}
	found := false
	oci := false
	//  we need to remove the oci:// prefix (in case it exists), because helmfile doesn't support the scheme in the repo url for oci based repositories.
	//  for these repositories, only url without a scheme and the oci: true flag is needed.
	d.Repository = strings.TrimPrefix(d.Repository, "oci://")
	if envctx.Requirements != nil {
		oci = envctx.Requirements.Cluster.ChartKind == jxcore.ChartRepositoryTypeOCI
	}
	prefixes := map[string]string{}
	urls := map[string]string{}
	for _, appsConfig := range helmStates {
		for k := range appsConfig.Repositories {
			r := appsConfig.Repositories[k]
			if r.URL == d.Repository {
				found = true
				r.OCI = oci
			}
			if r.Name != "" {
				urls[r.URL] = r.Name
				prefixes[r.Name] = r.URL
			}
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
		appsConfig := helmStates[len(helmStates)-1]
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
