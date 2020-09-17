package environments

import (
	"io/ioutil"
	"os"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/pkg/termcolor"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx-logging/pkg/log"
)

const (
	// LabelUpdatebot is the label applied to PRs created by updatebot
	LabelUpdatebot = "updatebot"
)

// Create a pull request against the environment repository for env.
// The EnvironmentPullRequestOptions are used to provide a Gitter client for performing git operations,
// a GitProvider client for talking to the git provider,
// a callback ModifyChartFn which is where the changes you want to make are defined.
// The branchNameText defines the branch name used, the title is used for both the commit and the pull request title,
// the message as the body for both the commit and the pull request,
// and the pullRequestInfo for any existing PR that exists to modify the environment that we want to merge these
// changes into.
func (o *EnvironmentPullRequestOptions) Create(gitURL, prDir string, pullRequestDetails *scm.PullRequest, autoMerge bool) (*scm.PullRequest, error) {
	if prDir == "" {
		tempDir, err := ioutil.TempDir("", "create-pr")
		if err != nil {
			return nil, err
		}
		prDir = tempDir
		defer os.RemoveAll(tempDir)
	}

	dir, err := gitclient.CloneToDir(o.Gitter, gitURL, "")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clone git URL %s", gitURL)
	}

	o.OutDir = dir
	log.Logger().Infof("cloned %s to %s", termcolor.ColorInfo(gitURL), termcolor.ColorInfo(dir))

	// TODO fork if needed?
	currentSha, err := gitclient.GetLatestCommitSha(o.Gitter, dir)
	if err != nil {
		return nil, errors.Wrap(err, "could not get current commit sha")
	}

	if o.Function == nil {
		return nil, errors.Errorf("no change function configured")
	}
	err = o.Function()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to invoke change function in dir %s", dir)
	}

	labels := make([]string, 0)
	labels = append(labels, o.Labels...)
	if autoMerge {
		value := LabelUpdatebot
		contains := false
		for _, l := range pullRequestDetails.Labels {
			if l != nil {
				if l.Name == value {
					contains = true
					break
				}
			}
		}
		if !contains {
			pullRequestDetails.Labels = append(pullRequestDetails.Labels, &scm.Label{
				Name: value,
			})
		}
	}

	latestSha, err := gitclient.GetLatestCommitSha(o.Gitter, dir)
	if err != nil {
		return nil, errors.Wrap(err, "could not get current latest commit sha")
	}

	doneCommit := true
	if latestSha == currentSha {
		changed, err := gitclient.HasChanges(o.Gitter, dir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to detect changes in dir %s", dir)
		}
		if !changed {
			// lets avoid failing to create the PR as we really have made changes
			doneCommit = false
		}
	}

	prInfo, err := o.CreatePullRequest(dir, gitURL, o.GitKind, doneCommit)
	if err != nil {
		return prInfo, errors.Wrapf(err, "failed to create pull request in dir %s", dir)
	}
	return prInfo, nil
}
