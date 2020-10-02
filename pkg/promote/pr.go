package promote

import (
	"fmt"
	"path/filepath"

	"github.com/jenkins-x/go-scm/scm"
	v1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	api_config "github.com/jenkins-x/jx-api/pkg/config"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/gitconfig"
	"github.com/jenkins-x/jx-helpers/pkg/yaml2s"
	"github.com/jenkins-x/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx-promote/pkg/rules/factory"
	"github.com/jenkins-x/jx-promote/pkg/rules/helmfile"
	"github.com/pkg/errors"
)

func (o *Options) PromoteViaPullRequest(env *v1.Environment, releaseInfo *ReleaseInfo, draftPR bool) error {
	configureDependencyMatrix()

	version := o.Version
	versionName := version
	if versionName == "" {
		versionName = "latest"
	}
	app := o.Application

	envName := env.Spec.Label
	if envName == "" {
		envName = env.Name
	}
	details := scm.PullRequest{
		Source: "promote-" + app + "-" + versionName + "-" + env.Name,
		Title:  fmt.Sprintf("chore: promote %s to version %s in %s environment", app, versionName, envName),
		Body:   fmt.Sprintf("chore: promote %s to version %s in %s environment", app, versionName, envName),
		Draft:  draftPR,
		Labels: []*scm.Label{
			{
				Name:        "env/" + env.Name,
				Description: envName,
			},
		},
	}

	if draftPR {
		details.Labels = append(details.Labels, &scm.Label{
			Name:        "do-not-merge/hold",
			Description: "do not merge yet",
		})
	}

	o.EnvironmentPullRequestOptions.CommitTitle = details.Title
	o.EnvironmentPullRequestOptions.CommitMessage = details.Body

	envDir := ""
	if o.CloneDir != "" {
		envDir = o.CloneDir
	}

	promoteNS := ""
	if o.DevEnvContext.DevEnv != nil && o.DevEnvContext.DevEnv.Spec.Source.URL == env.Spec.Source.URL {
		promoteNS = env.Spec.Namespace
	}
	if env.Spec.RemoteCluster == true {
		ns, err := getRemoteNamespace(o, env, app)
		if err != nil {
			return err
		}
		if ns != nil {
			promoteNS = *ns
		}
	}

	o.Function = func() error {
		configureDependencyMatrix()

		dir := o.OutDir
		promoteConfig, _, err := promoteconfig.Discover(dir, promoteNS)
		if err != nil {
			return errors.Wrapf(err, "failed to discover the PromoteConfig in dir %s", dir)
		}

		r := &rules.PromoteRule{
			TemplateContext: rules.TemplateContext{
				GitURL:            "",
				Version:           o.Version,
				AppName:           o.Application,
				ChartAlias:        o.Alias,
				Namespace:         o.Namespace,
				HelmRepositoryURL: o.HelmRepositoryURL,
			},
			Dir:           dir,
			Config:        *promoteConfig,
			DevEnvContext: &o.DevEnvContext,
		}

		// lets check if we need the apps git URL
		if promoteConfig.Spec.FileRule != nil || promoteConfig.Spec.KptRule != nil {
			if o.AppGitURL == "" {
				_, gitConf, err := gitclient.FindGitConfigDir("")
				if err != nil {
					return errors.Wrapf(err, "failed to find git config dir")
				}
				o.AppGitURL, err = gitconfig.DiscoverUpstreamGitURL(gitConf)
				if err != nil {
					return errors.Wrapf(err, "failed to discover application git URL")
				}
				if o.AppGitURL == "" {
					return errors.Errorf("could not to discover application git URL")
				}
			}
			r.TemplateContext.GitURL = o.AppGitURL
		}

		fn := factory.NewFunction(r)
		if fn == nil {
			return errors.Errorf("could not create rule function ")
		}
		return fn(r)
	}

	if releaseInfo.PullRequestInfo != nil {
		o.PullRequestNumber = releaseInfo.PullRequestInfo.Number
	}
	gitURL := env.Spec.Source.URL
	autoMerge := true
	if draftPR {
		autoMerge = false
	}
	info, err := o.Create(gitURL, envDir, &details, autoMerge)
	releaseInfo.PullRequestInfo = info
	return err
}

func configureDependencyMatrix() {
	// lets configure the dependency matrix path
	// TODO
	//dependencymatrix.DependencyMatrixDirName = filepath.Join(".jx", "dependencies")
}

func getRemoteNamespace(o *Options, env *v1.Environment, app string) (*string, error) {
	var promoteNS *string = nil
	// 1. Load helmfile
	hf := filepath.Join(o.OutDir, "helmfile.yaml")
	helmfile, err := helmfile.LoadHelmfile(hf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load helmfile for %s environment", env.GetName())
	}

	// 2. Check if app exists
	foundApp := false
	for i := range helmfile.Releases {
		release := &helmfile.Releases[i]
		if release.Name == app {
			foundApp = true
		}
	}

	// 3. If not found figure out proper namespace
	if !foundApp {
		// 3.1 Running with default app namespace
		if o.DefaultAppNamespace != "" {
			promoteNS = &o.DefaultAppNamespace
		} else { // 3.2 Load namespace from `jx-requirements.yml`
			promoteNS, err = getNamespaceFromRequirements(o.OutDir)
			if err != nil {
				return nil, err
			}
		}
	}

	return promoteNS, nil
}

func getNamespaceFromRequirements(outdir string) (*string, error) {
	path := filepath.Join(outdir, "jx-requirements.yml")
	state := api_config.RequirementsConfig{}
	err := yaml2s.LoadFile(path, state)
	if err != nil {
		return nil, err
	}
	return &state.Cluster.Namespace, nil
}
