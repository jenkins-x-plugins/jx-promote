package promote

import (
	"fmt"

	"github.com/jenkins-x/go-scm/scm"
	v1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/gitconfig"
	"github.com/jenkins-x/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx-promote/pkg/rules/factory"
	"github.com/pkg/errors"
)

func (o *Options) PromoteViaPullRequest(env *v1.Environment, releaseInfo *ReleaseInfo) error {
	configureDependencyMatrix()

	version := o.Version
	versionName := version
	if versionName == "" {
		versionName = "latest"
	}
	app := o.Application

	details := scm.PullRequest{
		Source: "promote-" + app + "-" + versionName,
		Title:  "chore: " + app + " to " + versionName,
		Body:   fmt.Sprintf("chore: Promote %s to version %s", app, versionName),
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
	info, err := o.Create(env, envDir, &details, "", true)
	releaseInfo.PullRequestInfo = info
	return err
}

func configureDependencyMatrix() {
	// lets configure the dependency matrix path
	// TODO
	//dependencymatrix.DependencyMatrixDirName = filepath.Join(".jx", "dependencies")
}
