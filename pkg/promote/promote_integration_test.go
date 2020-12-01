// +build integration

package promote_test

import (
	"context"
	"strings"
	"testing"

	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	v1fake "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner/fakerunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/stringhelpers"
	"github.com/jenkins-x/jx-promote/pkg/jxtesthelpers"
	"github.com/jenkins-x/jx-promote/pkg/promote"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
)

// PromoteTestCase a test case of promote
type PromoteTestCase struct {
	name   string
	gitURL string
	gitRef string
	remote bool
}

func TestPromoteIntegrationHelmfile(t *testing.T) {
	AssertPromoteIntegration(t, PromoteTestCase{
		gitURL: "https://github.com/jx3-gitops-repositories/jx3-gke-terraform-vault",
	})
}

func TestPromoteIntegrationMakefileKpt(t *testing.T) {
	AssertPromoteIntegration(t, PromoteTestCase{
		gitURL: "https://github.com/jstrachan/env-test-promote-makefile",
	})
}

// AssertPromoteIntegration asserts the test cases work
func AssertPromoteIntegration(t *testing.T, testCases ...PromoteTestCase) {
	version := "1.2.3"
	appName := "myapp"
	envName := "staging"
	ns := "jx"

	runner := NewFakeRunnerWithGitClone()

	for _, tc := range testCases {
		_, po := promote.NewCmdPromote()
		name := tc.name
		if name == "" {
			name = tc.gitURL
		}
		po.DisableGitConfig = true
		po.Application = appName
		po.Version = version
		po.Environment = envName

		po.NoPoll = true
		po.BatchMode = true
		po.GitKind = "fake"
		po.CommandRunner = runner.Run
		po.AppGitURL = "https://github.com/myorg/myapp.git"

		targetFullName := "jenkins-x/default-environment-helmfile"

		devEnv, err := jxtesthelpers.CreateTestDevEnvironment(ns)
		require.NoError(t, err, "failed to create dev environment")

		kubeObjects := []runtime.Object{
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
					Labels: map[string]string{
						"tag":  "",
						"team": "jx",
						"env":  "dev",
					},
				},
			},
		}
		jxObjects := []runtime.Object{
			devEnv,
			&v1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envName,
					Namespace: ns,
				},
				Spec: v1.EnvironmentSpec{
					Label:             strings.Title(envName),
					Namespace:         "jx-" + envName,
					PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
					Source: v1.EnvironmentRepository{
						Kind: v1.EnvironmentRepositoryTypeGit,
						URL:  tc.gitURL,
						Ref:  tc.gitRef,
					},
					Order:          0,
					Kind:           "",
					PullRequestURL: "",
					TeamSettings:   v1.TeamSettings{},
					RemoteCluster:  tc.remote,
				},
			},
		}

		po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
		po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
		po.Namespace = ns
		po.Build = "1"
		po.Pipeline = "myorg/myapp/master"
		po.DevEnvContext.VersionResolver = jxtesthelpers.CreateTestVersionResolver(t)

		err = po.Run()
		require.NoError(t, err, "failed test %s s", name)

		scmClient := po.ScmClient
		require.NotNil(t, scmClient, "no ScmClient created")
		ctx := context.Background()
		pr, _, err := scmClient.PullRequests.Find(ctx, targetFullName, 1)
		require.NoError(t, err, "failed to find repository %s", targetFullName)
		assert.NotNil(t, pr, "nil pr %s", targetFullName)

		t.Logf("created PullRequest %s", pr.Link)
		t.Logf("PR title: %s", pr.Title)
		t.Logf("PR body: %s", pr.Body)

		// lets assert we have a PipelineActivity...
		paList, err := po.JXClient.CoreV4beta1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
		require.NoError(t, err, "failed to load PipelineActivity resources in namespace %s", ns)
		require.Len(t, paList.Items, 1, "should have a PipelineActivity in namespace %s", ns)
		pa := paList.Items[0]

		data, err := yaml.Marshal(pa)
		require.NoError(t, err, "failed to marshal PipelineActivity")

		t.Logf("got PipelineActivity %s\n", string(data))
		assert.Equal(t, v1.ActivityStatusTypeSucceeded, pa.Spec.Status, "pipelineActivity.Spec.Status")
	}
}

func NewFakeRunnerWithGitClone() *fakerunner.FakeRunner {
	runner := &fakerunner.FakeRunner{}

	validGitCommands := []string{"clone", "rev-parse", "status"}

	runner.CommandRunner = func(c *cmdrunner.Command) (string, error) {
		if c.Name == "git" && len(c.Args) > 0 && stringhelpers.StringArrayIndex(validGitCommands, c.Args[0]) >= 0 {
			// lets really perform the git command
			return cmdrunner.DefaultCommandRunner(c)
		}

		// lets fake out other commands
		return "", nil
	}
	return runner
}
