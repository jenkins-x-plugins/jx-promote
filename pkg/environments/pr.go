package environments

import (
	"context"
	"fmt"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-promote/pkg/authhelpers"
	"github.com/jenkins-x/jx-promote/pkg/githelpers"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// Git lazily create a gitter if its not specified
func (o *EnvironmentPullRequestOptions) Git() gits.Gitter {
	if o.Gitter == nil {
		o.Gitter = gits.NewGitCLI()
	}
	return o.Gitter
}

// CreatePullRequest crates a pull request if there are git changes
func (o *EnvironmentPullRequestOptions) CreatePullRequest(dir string, gitURL string, kind string, doneCommit bool) (*scm.PullRequest, error) {
	if gitURL == "" {
		log.Logger().Infof("no git URL specified so cannot create a Pull Request. Changes have been saved to %s", dir)
		return nil, nil
	}

	gitter := o.Git()
	changes, err := gitter.HasChanges(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to detect if there were git changes in dir %s", dir)
	}
	if !changes && !doneCommit {
		log.Logger().Infof("no changes detected so not creating a Pull Request on %s", util.ColorInfo(gitURL))
		return nil, nil
	}

	if o.BranchName == "" {
		o.BranchName, err = githelpers.CreateBranch(gitter, dir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create git branch in %s", dir)
		}
	}

	commitTitle := strings.TrimSpace(o.CommitTitle)
	commitBody := o.commitBody.String()

	commitMessageStart := o.CommitMessage
	if commitMessageStart == "" {
		commitMessageStart = commitTitle
	}
	commitMessage := fmt.Sprintf("%s\n\n%s", commitMessageStart, commitBody)
	err = gitter.AddCommit(dir, commitMessage)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to commit changes in dir %s", dir)
	}

	remote := "origin"
	err = gitter.Push(dir, remote, false)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to push to remote %s from dir %s", remote, dir)
	}

	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse git URL")
	}

	serverURL := gitInfo.HostURLWithoutUser()
	owner := gitInfo.Organisation

	scmClient, _, err := o.CreateScmClient(serverURL, owner, kind)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create SCM client for %s", gitURL)
	}
	o.ScmClient = scmClient
	ctx := context.Background()

	headPrefix := ""
	if o.Fork {
		user, _, err := scmClient.Users.Find(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find current SCM user")
		}

		username := user.Login
		headPrefix = username + ":"
	}

	head := headPrefix + o.BranchName

	pri := &scm.PullRequestInput{
		Title: commitTitle,
		Head:  head,
		Base:  "master",
		Body:  commitBody,
	}
	repoFullName := scm.Join(gitInfo.Organisation, gitInfo.Name)
	pr, _, err := scmClient.PullRequests.Create(ctx, repoFullName, pri)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create PullRequest on %s", gitURL)
	}

	// the URL should not really end in .diff - fix in go-scm
	link := strings.TrimSuffix(pr.Link, ".diff")
	pr.Link = link
	log.Logger().Infof("created Pull Request %s from dir %s", util.ColorInfo(link), util.ColorInfo(dir))
	return pr, nil
}

// CreateScmClient creates a new scm client
func (o *EnvironmentPullRequestOptions) CreateScmClient(gitServer, owner, gitKind string) (*scm.Client, string, error) {
	af, err := authhelpers.NewAuthFacadeWithArgs(o.AuthConfigService, o.Git(), o.IOFileHandles, o.BatchMode, o.UseGitHubOAuth)
	if err != nil {
		return nil, "", errors.Wrapf(err, "failed to create git auth facade")
	}
	scmClient, token, _, err := af.ScmClient(gitServer, owner, gitKind)
	if err != nil {
		return scmClient, token, errors.Wrapf(err, "failed to create SCM client for server %s", gitServer)
	}
	return scmClient, token, nil
}
