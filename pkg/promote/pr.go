package promote

import (
	"fmt"

	"github.com/jenkins-x/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx-promote/pkg/rules/factory"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/gits"
)

func (o *Options) PromoteViaPullRequest(env *v1.Environment, releaseInfo *ReleaseInfo) error {
	version := o.Version
	versionName := version
	if versionName == "" {
		versionName = "latest"
	}
	app := o.Application

	details := gits.PullRequestDetails{
		BranchName: "promote-" + app + "-" + versionName,
		Title:      "chore: " + app + " to " + versionName,
		Message:    fmt.Sprintf("chore: Promote %s to version %s", app, versionName),
	}

	o.EnvironmentPullRequestOptions.CommitTitle = details.Title
	o.EnvironmentPullRequestOptions.CommitMessage = details.Message

	envDir := ""
	if o.CloneDir != "" {
		envDir = o.CloneDir
	}

	o.Function = func() error {
		dir := o.OutDir
		promoteConfig, _, err := promoteconfig.Discover(dir)
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
				_, gitConf, err := o.Git().FindGitConfigDir("")
				if err != nil {
					return errors.Wrapf(err, "failed to find git config dir")
				}
				o.AppGitURL, err = o.Git().DiscoverUpstreamGitURL(gitConf)
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

	filter := &gits.PullRequestFilter{}
	if releaseInfo.PullRequestInfo != nil {
		filter.Number = &releaseInfo.PullRequestInfo.Number
	}
	info, err := o.Create(env, envDir, &details, filter, "", true)
	releaseInfo.PullRequestInfo = info
	return err
}
