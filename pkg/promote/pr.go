package promote

import (
	"fmt"

	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/gitconfig"
	"github.com/jenkins-x/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx-promote/pkg/rules/factory"
	"github.com/pkg/errors"
)

func (o *Options) PromoteViaPullRequest(env *v1.Environment, releaseInfo *ReleaseInfo, draftPR bool) error {
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
	comment := fmt.Sprintf("chore: promote %s to version %s in %s", app, versionName, envName) + "\n\nthis commit will trigger a pipeline to [generate the actual kubernetes resources to perform the promotion](https://jenkins-x.io/docs/v3/about/how-it-works/#promotion) which will create a second commit on this Pull Request before it can merge"
	details := scm.PullRequest{
		Source: "promote-" + app + "-" + versionName + "-" + env.Name,
		Title:  fmt.Sprintf("chore: promote %s to version %s in %s", app, versionName, envName),
		Body:   comment,
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

	promoteNS := env.Spec.Namespace

	o.Function = func() error {
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
	autoMerge := o.AutoMerge
	if draftPR {
		autoMerge = false
	}
	info, err := o.Create(gitURL, envDir, &details, autoMerge)
	releaseInfo.PullRequestInfo = info
	return err
}
