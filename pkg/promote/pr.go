package promote

import (
	"fmt"
	"os"

	"github.com/jenkins-x-plugins/jx-promote/pkg/environments"
	"github.com/jenkins-x/jx-helpers/v3/pkg/requirements"

	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"

	"github.com/jenkins-x-plugins/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/factory"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/gitconfig"
	"github.com/pkg/errors"
)

func (o *Options) PromoteViaPullRequest(envs []*jxcore.EnvironmentConfig, releaseInfo *ReleaseInfo, draftPR bool) error {
	version := o.Version
	versionName := version
	if versionName == "" {
		versionName = "latest"
	}
	app := o.Application

	source := "promote-" + app + "-" + versionName
	var labels []string

	for _, env := range envs {
		envName := env.Key
		source += "-" + envName
		labels = append(labels, "env/"+envName)
	}

	var dependencyLabel = "dependency/" + releaseInfo.FullAppName

	if len(dependencyLabel) > 49 {
		dependencyLabel = dependencyLabel[:49]
	}
	labels = append(labels, dependencyLabel)

	if o.ReusePullRequest && o.PullRequestFilter == nil {
		o.PullRequestFilter = &environments.PullRequestFilter{Labels: labels}
		// Clearing so that it can be set for the correct environment on next call
		defer func() { o.PullRequestFilter = nil }()
	}

	comment := "this commit will trigger a pipeline to [generate the actual kubernetes resources to perform the promotion](https://jenkins-x.io/docs/v3/about/how-it-works/#promotion) which will create a second commit on this Pull Request before it can merge"

	if draftPR {
		labels = append(labels, "do-not-merge/hold")
	}

	o.EnvironmentPullRequestOptions.CommitTitle = fmt.Sprintf("chore: promote %s to version %s", app, versionName)
	o.EnvironmentPullRequestOptions.CommitMessage = comment
	if o.AddChangelog != "" {
		changelog, err := os.ReadFile(o.AddChangelog)
		if err != nil {
			return errors.Wrapf(err, "failed to read changelog file %s", o.AddChangelog)
		}
		o.EnvironmentPullRequestOptions.CommitChangelog = string(changelog)
	}

	envDir := ""
	if o.CloneDir != "" {
		envDir = o.CloneDir
	}

	o.Function = func() error {
		dir := o.OutDir

		for _, env := range envs {
			promoteNS := EnvironmentNamespace(env)
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
					ReleaseName:       o.ReleaseName,
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
					o.AppGitURL, err = gitconfig.DiscoverUpstreamGitURL(gitConf, true)
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
			err = fn(r)
			if err != nil {
				return errors.Wrapf(err, "failed to promote to %s", env.Key)
			}
		}
		return nil
	}

	if releaseInfo.PullRequestInfo != nil {
		o.PullRequestNumber = releaseInfo.PullRequestInfo.Number
	}
	env := envs[0]
	gitURL := requirements.EnvironmentGitURL(o.DevEnvContext.Requirements, env.Key)
	if gitURL == "" {
		if env.RemoteCluster {
			return errors.Errorf("no git URL for remote cluster %s", env.Key)
		}

		// lets default to the git repository for the dev environment for local clusters
		gitURL = requirements.EnvironmentGitURL(o.DevEnvContext.Requirements, "dev")
		if gitURL == "" {
			return errors.Errorf("no git URL for dev environment")
		}
	}
	autoMerge := o.AutoMerge
	if draftPR {
		autoMerge = false
	}
	info, err := o.Create(gitURL, envDir, labels, autoMerge)
	releaseInfo.PullRequestInfo = info
	return err
}
