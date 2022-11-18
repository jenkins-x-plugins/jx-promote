package environments

import (
	"context"
	"os"
	"sort"

	"github.com/davecgh/go-spew/spew"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/maps"
	"github.com/jenkins-x/jx-helpers/v3/pkg/scmhelpers"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
func (o *EnvironmentPullRequestOptions) Create(gitURL, prDir string, labels []string, autoMerge bool) (*scm.PullRequest, error) {
	scmClient, repoFullName, err := o.GetScmClient(gitURL, o.GitKind)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create ScmClient")
	}
	if scmClient == nil {
		return nil, nil
	}

	existingPr, err := o.FindExistingPullRequest(scmClient, repoFullName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find existing PullRequest")
	}

	if prDir == "" {
		tempDir, err := os.MkdirTemp("", "create-pr")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tempDir)
	}

	cloneGitURL := gitURL
	if o.Fork {
		cloneGitURL, err = o.EnsureForked(scmClient, repoFullName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to ensure repository is forked %s", gitURL)
		}
	}
	cloneGitURLSafe := cloneGitURL
	if o.ScmClientFactory.GitToken != "" && o.ScmClientFactory.GitUsername != "" {
		cloneGitURL, err = o.ScmClientFactory.CreateAuthenticatedURL(cloneGitURL)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create authenticated git URL to clone with for private repositories")
		}
	}

	var dir string
	if len(o.SparseCheckoutPatterns) > 0 {
		dir, err = gitclient.SparseCloneToDir(o.Gitter, cloneGitURL, "", true, o.SparseCheckoutPatterns...)
	} else {
		dir, err = gitclient.CloneToDir(o.Gitter, cloneGitURL, "")
		if o.BaseBranchName != "" {
			log.Logger().Infof("checking out remote base branch %s from %s", o.BaseBranchName, gitURL)
			err = gitclient.CheckoutRemoteBranch(o.Gitter, dir, o.BaseBranchName)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to checkout remote branch %s from %s", o.BaseBranchName, gitURL)
			}
		}
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clone git URL %s", cloneGitURLSafe)
	}

	if o.Fork {
		err = o.rebaseForkFromUpstream(dir, gitURL)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to rebase forked repository")
		}
	}

	o.OutDir = dir
	log.Logger().Debugf("cloned %s to %s", termcolor.ColorInfo(cloneGitURLSafe), termcolor.ColorInfo(dir))

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

	o.Labels = nil

	// lets merge any labels together...
	labelsSet := make(map[string]string)
	if autoMerge {
		labelsSet[LabelUpdatebot] = ""
	}
	for _, l := range labels {
		if l != "" {
			labelsSet[l] = ""
		}
	}
	o.Labels = maps.MapKeys(labelsSet)

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

	prInfo, err := o.CreatePullRequest(scmClient, gitURL, repoFullName, dir, doneCommit, existingPr)
	if err != nil {
		return prInfo, errors.Wrapf(err, "failed to create pull request in dir %s", dir)
	}
	return prInfo, nil
}

func (o *EnvironmentPullRequestOptions) FindExistingPullRequest(scmClient *scm.Client, repoFullName string) (*scm.PullRequest, error) {
	if o.PullRequestFilter == nil {
		return nil, nil
	}
	ctx := context.Background()
	filterLabels := o.PullRequestFilter.Labels
	log.Logger().Debugf("Trying to find open PRs in %s with labels %v", repoFullName, filterLabels)
	prs, _, err := scmClient.PullRequests.List(ctx, repoFullName, &scm.PullRequestListOptions{
		Size:   100,
		Open:   true,
		Labels: filterLabels,
	})
	if scmhelpers.IsScmNotFound(err) || len(prs) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "Error listing PRs")
	}
	if log.Logger().Logger.IsLevelEnabled(logrus.TraceLevel) {
		log.Logger().Tracef("Found PRs: %s", spew.Sdump(prs))
	}

	// sort in descending order of PR numbers (assumes PRs numbers increment!)
	sort.Slice(prs, func(i, j int) bool {
		pi := prs[i]
		pj := prs[j]
		return pi.Number > pj.Number
	})

	// let's find the latest PR which is not closed
Prs:
	for i := range prs {
		pr := prs[i]
		if pr.Closed || pr.Merged || pr.Base.Repo.FullName != repoFullName {
			continue
		}
		for _, label := range filterLabels {
			if !scmhelpers.ContainsLabel(pr.Labels, label) {
				continue Prs
			}
		}
		log.Logger().Debugf("Found matching pr: %s", spew.Sdump(pr))
		return pr, nil
	}
	return nil, nil
}
