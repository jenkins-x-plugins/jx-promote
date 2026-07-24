//go:build unit
// +build unit

package envctx

import (
	"os"
	"path/filepath"
	"testing"

	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	v1fake "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jxenv"
	"github.com/jenkins-x/jx-helpers/v3/pkg/versionstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testNS = "jx"

// requirementsWithDevGitURL builds a RequirementsConfig with a single "dev"
// environment whose git URL is set to the given value.
func requirementsWithDevGitURL(gitURL string) *jxcore.RequirementsConfig {
	return &jxcore.RequirementsConfig{
		Environments: []jxcore.EnvironmentConfig{
			{
				Key:    "dev",
				GitURL: gitURL,
			},
		},
	}
}

func TestOverrideDevGitURL_AppliesRequirementsURL(t *testing.T) {
	c := &EnvironmentContext{
		DevEnv:       jxenv.CreateDefaultDevEnvironment(testNS),
		Requirements: requirementsWithDevGitURL("https://github.com/myorg/dev-cluster.git"),
	}
	c.DevEnv.Spec.Source.URL = "https://github.com/myorg/original.git"

	c.overrideDevGitURL()

	assert.Equal(t, "https://github.com/myorg/dev-cluster.git", c.DevEnv.Spec.Source.URL,
		"the requirements dev git URL should override the dev environment source URL")
}

func TestOverrideDevGitURL_NoOverrideWhenRequirementsEmpty(t *testing.T) {
	c := &EnvironmentContext{
		DevEnv:       jxenv.CreateDefaultDevEnvironment(testNS),
		Requirements: &jxcore.RequirementsConfig{},
	}
	c.DevEnv.Spec.Source.URL = "https://github.com/myorg/original.git"

	c.overrideDevGitURL()

	assert.Equal(t, "https://github.com/myorg/original.git", c.DevEnv.Spec.Source.URL,
		"the dev environment source URL should be unchanged when requirements have no dev git URL")
}

func TestLoadDevEnv_NoOpWhenAlreadyLoaded(t *testing.T) {
	existing := jxenv.CreateDefaultDevEnvironment(testNS)
	existing.Spec.Source.URL = "https://github.com/myorg/preset.git"

	c := &EnvironmentContext{DevEnv: existing}
	// an empty client would error if it was consulted
	jxClient := v1fake.NewSimpleClientset()

	err := c.loadDevEnv(jxClient, testNS)

	require.NoError(t, err)
	assert.Same(t, existing, c.DevEnv, "the preset dev environment must not be replaced")
}

func TestLoadDevEnv_LoadsFromClient(t *testing.T) {
	devEnv := jxenv.CreateDefaultDevEnvironment(testNS)
	devEnv.Namespace = testNS
	devEnv.Spec.Source.URL = "https://github.com/myorg/dev-cluster.git"

	c := &EnvironmentContext{}
	jxClient := v1fake.NewSimpleClientset(devEnv)

	err := c.loadDevEnv(jxClient, testNS)

	require.NoError(t, err)
	require.NotNil(t, c.DevEnv)
	assert.Equal(t, "https://github.com/myorg/dev-cluster.git", c.DevEnv.Spec.Source.URL)
}

func TestLoadDevEnv_ErrorWhenMissing(t *testing.T) {
	c := &EnvironmentContext{}
	jxClient := v1fake.NewSimpleClientset()

	err := c.loadDevEnv(jxClient, testNS)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no dev environment in namespace jx")
}

func TestLoadRequirements_NoOpWhenAlreadyLoaded(t *testing.T) {
	preset := requirementsWithDevGitURL("https://github.com/myorg/dev-cluster.git")

	c := &EnvironmentContext{Requirements: preset}
	// an empty client would error if FindRequirements was consulted
	jxClient := v1fake.NewSimpleClientset()

	err := c.loadRequirements(nil, jxClient, testNS, "")

	require.NoError(t, err)
	assert.Same(t, preset, c.Requirements, "the preset requirements must not be replaced")
}

func TestLoadVersionResolver_NoOpWhenAlreadyLoaded(t *testing.T) {
	preset := &versionstream.VersionResolver{VersionsDir: "/some/version/stream"}

	c := &EnvironmentContext{VersionResolver: preset}

	// gitter is nil: if it were consulted the test would panic, proving the no-op
	err := c.loadVersionResolver(nil)

	require.NoError(t, err)
	assert.Same(t, preset, c.VersionResolver, "the preset version resolver must not be replaced")
}

func TestLoadVersionResolver_ErrorWhenNoSourceURL(t *testing.T) {
	devEnv := jxenv.CreateDefaultDevEnvironment(testNS)
	devEnv.Name = "my-dev-env"
	devEnv.Spec.Source.URL = ""

	c := &EnvironmentContext{DevEnv: devEnv}

	err := c.loadVersionResolver(nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have a source URL")
}

func TestResolveGitCredentials_NoOpWhenAlreadySet(t *testing.T) {
	c := &EnvironmentContext{
		GitUsername: "someuser",
		GitToken:    "sometoken",
	}

	// the git URL is deliberately unparseable: it must not be consulted because
	// credentials are already supplied
	err := c.resolveGitCredentials("://not-a-valid-url")

	require.NoError(t, err)
	assert.Equal(t, "someuser", c.GitUsername)
	assert.Equal(t, "sometoken", c.GitToken)
}

func TestEnsureVersionStreamDir_ReturnsExistingDir(t *testing.T) {
	cloneDir := t.TempDir()
	versionsDir := filepath.Join(cloneDir, "versionStream")
	require.NoError(t, os.MkdirAll(versionsDir, 0o755))

	c := &EnvironmentContext{}
	got, err := c.ensureVersionStreamDir(cloneDir, "https://github.com/myorg/dev-cluster.git")

	require.NoError(t, err)
	assert.Equal(t, versionsDir, got)
	assert.DirExists(t, got)
}

func TestEnsureVersionStreamDir_CreatesMissingDir(t *testing.T) {
	cloneDir := t.TempDir()
	versionsDir := filepath.Join(cloneDir, "versionStream")

	c := &EnvironmentContext{}
	got, err := c.ensureVersionStreamDir(cloneDir, "https://github.com/myorg/dev-cluster.git")

	require.NoError(t, err)
	assert.Equal(t, versionsDir, got)
	assert.DirExists(t, got, "the versionStream dir should be created when it does not exist")
}

// TestLazyLoad_NoOpWhenFullyPopulated exercises the orchestration path when every
// value is already loaded: no dev environment, requirements or version stream is
// fetched, but the requirements dev git URL is still applied to the dev environment.
func TestLazyLoad_NoOpWhenFullyPopulated(t *testing.T) {
	devEnv := jxenv.CreateDefaultDevEnvironment(testNS)
	devEnv.Spec.Source.URL = "https://github.com/myorg/original.git"

	c := &EnvironmentContext{
		DevEnv:          devEnv,
		Requirements:    requirementsWithDevGitURL("https://github.com/myorg/dev-cluster.git"),
		VersionResolver: &versionstream.VersionResolver{VersionsDir: "/some/version/stream"},
	}

	// nil clients/gitter would panic if any load path was reached
	err := c.LazyLoad(nil, nil, testNS, nil, "")

	require.NoError(t, err)
	assert.Equal(t, "https://github.com/myorg/dev-cluster.git", c.DevEnv.Spec.Source.URL,
		"LazyLoad should apply the requirements dev git URL override")
	assert.Equal(t, "/some/version/stream", c.VersionResolver.VersionsDir,
		"the preset version resolver must be retained")
}
