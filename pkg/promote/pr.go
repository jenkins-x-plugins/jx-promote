package promote

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/jenkins-x-plugins/jx-promote/pkg/environments"
	"github.com/jenkins-x/jx-helpers/v3/pkg/requirements"

	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"

	"github.com/jenkins-x-plugins/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x-plugins/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/factory"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/gitconfig"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
)

// sparsePatternForRule derives the git sparse-checkout pattern (gitignore syntax, root anchored)
// needed to materialize the target of a file/kpt promote rule so it works under a sparse clone.
// A FileRule modifies exactly its Path; a KptRule operates within Path/<app>. Returns "" if the
// target cannot be derived (in which case the caller should fail rather than clone incompletely).
func sparsePatternForRule(spec v1alpha1.PromoteSpec, appName string) string {
	if spec.FileRule != nil {
		p := strings.Trim(spec.FileRule.Path, "/")
		if p == "" {
			return ""
		}
		return "/" + p
	}
	if spec.KptRule != nil {
		target := strings.Trim(path.Join(spec.KptRule.Path, appName), "/")
		if target == "" {
			return ""
		}
		return "/" + target + "/"
	}
	return ""
}

// isSparseCheckout reports whether the git working tree in dir is in sparse-checkout
// mode. SparseCloneToDir enables it via `git sparse-checkout set`, which sets
// core.sparseCheckout=true; the full-clone fallback in Create() leaves it unset. We
// key the sparse expansion off this actual repo state rather than the request flags,
// so a fallback full clone (which already contains every path) is not treated as
// sparse.
func isSparseCheckout(gitter gitclient.Interface, dir string) bool {
	out, err := gitter.Command(dir, "config", "--get", "core.sparseCheckout")
	if err != nil {
		return false // key unset -> git exits non-zero -> not sparse
	}
	return strings.TrimSpace(out) == "true"
}

func (o *Options) PromoteViaPullRequest(envs []*jxcore.EnvironmentConfig, releaseInfo *ReleaseInfo, draftPR bool) error {
	version := o.Version
	versionName := version
	if versionName == "" {
		versionName = "latest"
	}
	app := o.Application

	source := "promote-" + app + "-" + versionName
	var labels []string

	// TODO: Support more labels. I'm thinking owner...
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

	comment := "this commit will trigger a pipeline to [generate the actual kubernetes resources to perform the promotion](https://jenkins-x.io/v3/about/how-it-works/#promotion) which will create a second commit on this Pull Request before it can merge"

	if draftPR {
		labels = append(labels, "do-not-merge/hold")
	}

	o.CommitTitle = fmt.Sprintf("chore: promote %s to version %s", app, versionName)
	o.CommitMessage = comment
	if o.AddChangelog != "" {
		changelog, err := os.ReadFile(o.AddChangelog)
		if err != nil {
			return fmt.Errorf("failed to read changelog file %s: %w", o.AddChangelog, err)
		}
		o.CommitChangelog = string(changelog)
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
				return fmt.Errorf("failed to discover the PromoteConfig in dir %s: %w", dir, err)
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
				// file/kpt rules write to paths not covered by the default sparse-checkout pattern
				// set. The promote config itself is available (the default patterns include .jx/), so
				// we can derive the rule's target path and expand the sparse checkout to include it
				// rather than failing. Gate on the repo's actual sparse-checkout state, not the
				// request flags: Create() may have fallen back to a full clone when sparse checkout
				// was unsupported, and that clone already contains the rule's target path.
				if isSparseCheckout(o.Gitter, dir) {
					pattern := sparsePatternForRule(promoteConfig.Spec, o.Application)
					if pattern == "" {
						return fmt.Errorf("promote config in dir %s uses a file/kpt rule whose target path cannot be derived; please pass explicit --sparse-checkout-pattern values (or omit --sparse-checkout)", dir)
					}
					// git sparse-checkout add appends the pattern and re-applies it to the working
					// tree, lazily fetching the now-included blobs from the (blobless) clone
					if _, err := o.Gitter.Command(dir, "sparse-checkout", "add", pattern); err != nil {
						return fmt.Errorf("failed to expand sparse checkout with file/kpt rule path %q in dir %s: %w", pattern, dir, err)
					}
					log.Logger().Infof("expanded sparse checkout to include file/kpt rule path %s", pattern)
				}
				if o.AppGitURL == "" {
					_, gitConf, err := gitclient.FindGitConfigDir("")
					if err != nil {
						return fmt.Errorf("failed to find git config dir: %w", err)
					}
					o.AppGitURL, err = gitconfig.DiscoverUpstreamGitURL(gitConf, true)
					if err != nil {
						return fmt.Errorf("failed to discover application git URL: %w", err)
					}
					if o.AppGitURL == "" {
						return fmt.Errorf("could not to discover application git URL")
					}
				}
				r.GitURL = o.AppGitURL
			}

			fn := factory.NewFunction(r)
			if fn == nil {
				return fmt.Errorf("could not create rule function ")
			}
			err = fn(r)
			if err != nil {
				return fmt.Errorf("failed to promote to %s: %w", env.Key, err)
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
			return fmt.Errorf("no git URL for remote cluster %s", env.Key)
		}

		// lets default to the git repository for the dev environment for local clusters
		gitURL = requirements.EnvironmentGitURL(o.DevEnvContext.Requirements, "dev")
		if gitURL == "" {
			return fmt.Errorf("no git URL for dev environment")
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
