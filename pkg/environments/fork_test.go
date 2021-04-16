package environments_test

import (
	"context"
	"github.com/jenkins-x-plugins/jx-promote/pkg/environments"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFork(t *testing.T) {
	scmClient, _ := fake.NewDefault()

	ctx := context.TODO()
	o := &environments.EnvironmentPullRequestOptions{
		Fork: true,
	}
	o.ScmClientFactory.ScmClient = scmClient

	org := "jenkins-x-plugins"
	repoName := "jx-promote"
	expected := "https://fake.com/fakeuser/jx-promote.git"
	fullName := scm.Join(org, repoName)
	forkFullName := scm.Join(scmClient.Username, repoName)

	gitURL, err := o.EnsureForked(scmClient, fullName)
	require.NoError(t, err, "failed to EnsureForked on first call")
	assert.Equal(t, expected, gitURL, "for first EnsureForked on first call on %s", fullName)

	repo, _, err := scmClient.Repositories.Find(ctx, forkFullName)
	require.NoError(t, err, "should have found repo %s", forkFullName)
	require.NotNil(t, repo, "should have found repository %s", forkFullName)
	assert.Equal(t, expected, repo.Clone, "forked repo %s clone URL", forkFullName)

	gitURL, err = o.EnsureForked(scmClient, fullName)
	require.NoError(t, err, "failed to EnsureForked on second call")
	assert.Equal(t, expected, gitURL, "for second EnsureForked on first call on %s", fullName)

	t.Logf("ensure forked to git URL %s\n", gitURL)
}
