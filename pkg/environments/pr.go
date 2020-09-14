package environments

import (
	"context"
	"fmt"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/cli"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/pkg/errors"
)

// Git lazily create a gitter if its not specified
func (o *EnvironmentPullRequestOptions) Git() gitclient.Interface {
	if o.Gitter == nil {
		o.Gitter = cli.NewCLIClient("", o.CommandRunner)
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
	changes, err := gitclient.HasChanges(gitter, dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to detect if there were git changes in dir %s", dir)
	}
	if !changes && !doneCommit {
		log.Logger().Infof("no changes detected so not creating a Pull Request on %s", termcolor.ColorInfo(gitURL))
		return nil, nil
	}

	if o.BranchName == "" {
		o.BranchName, err = gitclient.CreateBranch(gitter, dir)
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
	_, err = gitclient.AddAndCommitFiles(gitter, dir, commitMessage)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to commit changes in dir %s", dir)
	}

	err = gitclient.ForcePushBranch(gitter, dir, o.BranchName, o.BranchName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to push to branch %s from dir %s", o.BranchName, dir)
	}

	gitInfo, err := giturl.ParseGitURL(gitURL)
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
	log.Logger().Infof("created Pull Request %s from dir %s", termcolor.ColorInfo(link), termcolor.ColorInfo(dir))
	return pr, nil
}

// CreateScmClient creates a new scm client
func (o *EnvironmentPullRequestOptions) CreateScmClient(gitServer, owner, gitKind string) (*scm.Client, string, error) {
	o.ScmClientFactory.GitServerURL = gitServer
	o.ScmClientFactory.Owner = owner
	o.ScmClientFactory.GitKind = gitKind
	scmClient, err := o.ScmClientFactory.Create()
	if err != nil {
		return scmClient, "", errors.Wrapf(err, "failed to create SCM client for server %s", gitServer)
	}
	return scmClient, o.ScmClientFactory.GitToken, nil
}
