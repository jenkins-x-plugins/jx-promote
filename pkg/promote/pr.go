package promote

import (
	"fmt"

	"github.com/jenkins-x/jx-helpers/v3/pkg/requirements"

	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/gitconfig"
	"github.com/jenkins-x-plugins/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/factory"
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
	var labels []*scm.Label

	for _, env := range envs {
		envName := env.Key
		source += "-" + envName
		labels = append(labels, &scm.Label{
			Name:        "env/" + envName,
			Description: envName,
		})
	}

	comment := fmt.Sprintf("chore: promote %s to version %s", app, versionName) + "\n\nthis commit will trigger a pipeline to [generate the actual kubernetes resources to perform the promotion](https://jenkins-x.io/docs/v3/about/how-it-works/#promotion) which will create a second commit on this Pull Request before it can merge"
	details := scm.PullRequest{
		Source: source,
		Title:  fmt.Sprintf("chore: promote %s to version %s", app, versionName),
		Body:   comment,
		Draft:  draftPR,
		Labels: labels,
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
	info, err := o.Create(gitURL, envDir, &details, autoMerge)
	releaseInfo.PullRequestInfo = info
	return err
}
