package environments

import (
	"context"
	"fmt"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/scmhelpers"
)

// EnsureForked ensures that the git repository is forked
func (o *EnvironmentPullRequestOptions) EnsureForked(client *scm.Client, repoName string) (string, error) {
	ctx := context.TODO()
	_, localName := scm.Split(repoName)
	if localName == "" {
		return "", fmt.Errorf("no local name for repository %s", repoName)
	}
	createFork := false

	forkFullName := scm.Join(client.Username, localName)
	repo, _, err := client.Repositories.Find(ctx, forkFullName)
	if scmhelpers.IsScmNotFound(err) {
		err = nil
		createFork = true
	}
	if err != nil {
		return "", fmt.Errorf("failed to find repository %s: %w", forkFullName, err)
	}
	if !createFork && repo != nil {
		return repo.Clone, nil
	}

	input := &scm.RepositoryInput{
		Name: localName,
	}
	repo, _, err = client.Repositories.Fork(ctx, input, repoName)
	if err != nil {
		return "", fmt.Errorf("failed to fork repository %s: %w", repoName, err)
	}
	return repo.Clone, nil
}

func (o *EnvironmentPullRequestOptions) rebaseForkFromUpstream(dir, gitURL string) error {
	g := o.Git()
	branch, err := gitclient.Branch(g, dir)
	if err != nil {
		return fmt.Errorf("failed to find current branch: %w", err)
	}
	remoteName := "upstream"
	err = gitclient.AddRemote(g, dir, remoteName, gitURL)
	if err != nil {
		return fmt.Errorf("failed to add remote %s to %s: %w", remoteName, gitURL, err)
	}

	_, err = g.Command(dir, "pull", "-r", remoteName, branch)
	if err != nil {
		return fmt.Errorf("failed to rebase from %s: %w", gitURL, err)
	}
	return nil
}
