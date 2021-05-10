// +build integration

package promote_test

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-helpers/v3/pkg/yaml2s"
	"github.com/roboll/helmfile/pkg/state"
	"k8s.io/api/extensions/v1beta1"

	"github.com/jenkins-x-plugins/jx-promote/pkg/jxtesthelpers"
	"github.com/jenkins-x-plugins/jx-promote/pkg/promote"
	"github.com/jenkins-x/go-scm/scm"
	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"
	v1fake "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner/fakerunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/stringhelpers"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
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
		gitURL: "https://github.com/jx3-gitops-repositories/jx3-gke-vault",
	})
}

func TestPromoteIntegrationMakefileKpt(t *testing.T) {
	AssertPromoteIntegration(t, PromoteTestCase{
		gitURL: "https://github.com/jstrachan/env-test-promote-makefile",
	})
}

func TestPromoteToGitHubPagesChartRepository(t *testing.T) {
	version := "1.2.3"
	appName := "myapp"
	ns := "jx"

	runner := NewFakeRunnerWithGitClone()

	_, po := promote.NewCmdPromote()
	name := "promote-github-pages"
	po.Dir = filepath.Join("test_data", "ghpages")
	po.DisableGitConfig = true
	po.Application = appName
	po.Version = version
	po.All = true

	po.NoPoll = true
	po.BatchMode = true
	po.GitKind = "fake"
	po.CommandRunner = runner.Run
	po.AppGitURL = "https://github.com/myorg/myapp.git"

	targetFullName := "jenkins-x/default-environment-helmfile"

	devEnv := jxtesthelpers.CreateTestDevEnvironment(ns)
	devGitURL := "https://github.com/jenkins-x-labs-bdd-tests/jx3-kubernetes-jenkins"
	devEnv.Spec.Source.URL = devGitURL

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
	}

	po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
	po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
	po.Namespace = ns
	po.Build = "1"
	po.Pipeline = "myorg/myapp/master"
	po.DevEnvContext.VersionResolver = jxtesthelpers.CreateTestVersionResolver(t)
	po.DevEnvContext.Requirements = &jxcore.RequirementsConfig{
		Cluster: jxcore.ClusterConfig{
			DestinationConfig: jxcore.DestinationConfig{
				ChartRepository: "https://github.com/jenkins-x-bdd/mycharts",
				ChartKind:       "pages",
			},
		},
		Environments: []jxcore.EnvironmentConfig{
			{
				Key:               "dev",
				Namespace:         "jx",
				PromotionStrategy: v1.PromotionStrategyTypeNever,
				GitURL:            devGitURL,
			},
			{
				Key:               "staging",
				Namespace:         "jx-staging",
				PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
			},
		},
	}

	err := po.Run()
	require.NoError(t, err, "failed test %s s", name)

	require.NotEmpty(t, po.OutDir, "should have populated an out dir")

	helmfile := filepath.Join(po.OutDir, "helmfiles", "jx-staging", "helmfile.yaml")
	require.FileExists(t, helmfile, "should be able to find helmfile")

	helmState := &state.HelmState{}
	err = yaml2s.LoadFile(helmfile, helmState)
	require.NoError(t, err, "failed to load helmfile %s", helmfile)

	devRepo := ""
	for _, repo := range helmState.Repositories {
		if repo.Name == "dev" {
			devRepo = repo.URL
			break
		}
	}
	assert.Equal(t, "https://jenkins-x-bdd.github.io/mycharts/", devRepo, "promoted dev helm chart URL")

	scmClient := po.ScmClient
	require.NotNil(t, scmClient, "no ScmClient created")
	ctx := context.Background()

	prNumber := 1
	pr, _, err := scmClient.PullRequests.Find(ctx, targetFullName, prNumber)
	require.NoError(t, err, "failed to find repository %s number %d", targetFullName, prNumber)
	assert.NotNil(t, pr, "nil pr %s for %s", targetFullName, prNumber)

	t.Logf("created PullRequest %s #%d", pr.Link, prNumber)
	t.Logf("PR title: %s", pr.Title)
	t.Logf("PR body: %s", pr.Body)

	// lets assert we have a PipelineActivity...
	paList, err := po.JXClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err, "failed to load PipelineActivity resources in namespace %s", ns)
	require.Len(t, paList.Items, 1, "should have a PipelineActivity in namespace %s", ns)
	pa := paList.Items[0]

	data, err := yaml.Marshal(pa)
	require.NoError(t, err, "failed to marshal PipelineActivity")

	t.Logf("got PipelineActivity %s\n", string(data))
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, pa.Spec.Status, "pipelineActivity.Spec.Status")
}

func TestPromoteHelmfileAllAutomaticAndManual(t *testing.T) {
	version := "1.2.3"
	appName := "myapp"
	ns := "jx"

	runner := NewFakeRunnerWithGitClone()

	_, po := promote.NewCmdPromote()
	name := "promote-all"
	po.DisableGitConfig = true
	po.Application = appName
	po.Version = version
	po.All = true

	po.NoPoll = true
	po.BatchMode = true
	po.GitKind = "fake"
	po.CommandRunner = runner.Run
	po.AppGitURL = "https://github.com/myorg/myapp.git"

	targetFullName := "jenkins-x/default-environment-helmfile"

	devEnv := jxtesthelpers.CreateTestDevEnvironment(ns)
	devGitURL := "https://github.com/jenkins-x-labs-bdd-tests/jx3-kubernetes-jenkins"
	devEnv.Spec.Source.URL = devGitURL

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
	}

	po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
	po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
	po.Namespace = ns
	po.Build = "1"
	po.Pipeline = "myorg/myapp/master"
	po.DevEnvContext.VersionResolver = jxtesthelpers.CreateTestVersionResolver(t)
	po.DevEnvContext.Requirements = &jxcore.RequirementsConfig{
		Environments: []jxcore.EnvironmentConfig{
			{
				Key:               "dev",
				Namespace:         "jx",
				PromotionStrategy: v1.PromotionStrategyTypeNever,
				GitURL:            devGitURL,
			},
			{
				Key:               "staging",
				Namespace:         "jx-staging",
				PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
			},
			{
				Key:               "production",
				Namespace:         "jx-production",
				PromotionStrategy: v1.PromotionStrategyTypeManual,
			},
		},
	}

	err := po.Run()
	require.NoError(t, err, "failed test %s s", name)

	scmClient := po.ScmClient
	require.NotNil(t, scmClient, "no ScmClient created")
	ctx := context.Background()

	for prNumber := 1; prNumber < 3; prNumber++ {
		pr, _, err := scmClient.PullRequests.Find(ctx, targetFullName, prNumber)
		require.NoError(t, err, "failed to find repository %s number %d", targetFullName, prNumber)
		assert.NotNil(t, pr, "nil pr %s for %s", targetFullName, prNumber)

		t.Logf("created PullRequest %s #%d", pr.Link, prNumber)
		t.Logf("PR title: %s", pr.Title)
		t.Logf("PR body: %s", pr.Body)
	}
	// lets assert we have a PipelineActivity...
	paList, err := po.JXClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err, "failed to load PipelineActivity resources in namespace %s", ns)
	require.Len(t, paList.Items, 1, "should have a PipelineActivity in namespace %s", ns)
	pa := paList.Items[0]

	data, err := yaml.Marshal(pa)
	require.NoError(t, err, "failed to marshal PipelineActivity")

	t.Logf("got PipelineActivity %s\n", string(data))
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, pa.Spec.Status, "pipelineActivity.Spec.Status")
}

func TestPromoteHelmfileCustomNamespace(t *testing.T) {
	version := "1.2.3"
	appName := "myapp"
	ns := "jx"

	runner := NewFakeRunnerWithGitClone()

	_, po := promote.NewCmdPromote()
	name := "promote-all"
	po.DisableGitConfig = true
	po.Application = appName
	po.Version = version
	po.All = true

	po.NoPoll = true
	po.BatchMode = true
	po.GitKind = "fake"
	po.CommandRunner = runner.Run
	po.AppGitURL = "https://github.com/myorg/myapp.git"

	targetFullName := "jenkins-x/default-environment-helmfile"

	devEnv := jxtesthelpers.CreateTestDevEnvironment(ns)
	devGitURL := "https://github.com/jenkins-x-labs-bdd-tests/jx3-kubernetes-jenkins"
	devEnv.Spec.Source.URL = devGitURL

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
	}

	po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
	po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
	po.Namespace = ns
	po.Build = "1"
	po.Pipeline = "myorg/myapp/master"
	po.DevEnvContext.VersionResolver = jxtesthelpers.CreateTestVersionResolver(t)
	po.DevEnvContext.Requirements = &jxcore.RequirementsConfig{
		Environments: []jxcore.EnvironmentConfig{
			{
				Key:               "dev",
				Namespace:         "jx",
				PromotionStrategy: v1.PromotionStrategyTypeNever,
				GitURL:            devGitURL,
			},
			{
				Key:               "staging",
				Namespace:         "my-staging-ns",
				PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
			},
		},
	}

	err := po.Run()
	require.NoError(t, err, "failed test %s s", name)

	scmClient := po.ScmClient
	require.NotNil(t, scmClient, "no ScmClient created")
	ctx := context.Background()

	prNumber := 1
	pr, _, err := scmClient.PullRequests.Find(ctx, targetFullName, prNumber)
	require.NoError(t, err, "failed to find repository %s number %d", targetFullName, prNumber)
	assert.NotNil(t, pr, "nil pr %s for %s", targetFullName, prNumber)

	t.Logf("created PullRequest %s #%d", pr.Link, prNumber)
	t.Logf("PR title: %s", pr.Title)
	t.Logf("PR body: %s", pr.Body)

	// lets assert we have a PipelineActivity...
	paList, err := po.JXClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err, "failed to load PipelineActivity resources in namespace %s", ns)
	require.Len(t, paList.Items, 1, "should have a PipelineActivity in namespace %s", ns)
	pa := paList.Items[0]

	data, err := yaml.Marshal(pa)
	require.NoError(t, err, "failed to marshal PipelineActivity")

	t.Logf("got PipelineActivity %s\n", string(data))
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, pa.Spec.Status, "pipelineActivity.Spec.Status")
}

func TestPromoteHelmfileAllAutomaticsInOneOrMorePRs(t *testing.T) {
	targetFullName := "jenkins-x-labs-bdd-tests/jx3-kubernetes-jenkins"

	testCases := []struct {
		name                     string
		envSourceURL             string
		noGroupPullRequest       bool
		expectedPullRequestCount map[string]int
	}{
		{
			name:               "separate-prs-for-urls",
			noGroupPullRequest: false,
			envSourceURL:       "https://github.com/jx3-gitops-repositories/jx3-gke-vault",
			expectedPullRequestCount: map[string]int{
				targetFullName:                          1,
				"jx3-gitops-repositories/jx3-gke-vault": 1,
			},
		},
		{
			name:               "group-prs",
			noGroupPullRequest: false,
			expectedPullRequestCount: map[string]int{
				targetFullName: 1,
			},
		},
		{
			name:               "separate-prs",
			noGroupPullRequest: true,
			expectedPullRequestCount: map[string]int{
				targetFullName: 2,
			},
		},
	}

	for _, tc := range testCases {
		version := "1.2.3"
		appName := "myapp"
		ns := "jx"

		runner := NewFakeRunnerWithGitClone()

		_, po := promote.NewCmdPromote()
		name := tc.name
		po.DisableGitConfig = true
		po.Application = appName
		po.Version = version
		po.All = true

		po.NoPoll = true
		po.BatchMode = true
		po.NoGroupPullRequest = tc.noGroupPullRequest
		po.GitKind = "fake"
		po.CommandRunner = runner.Run
		po.AppGitURL = "https://github.com/myorg/myapp.git"

		devEnv := jxtesthelpers.CreateTestDevEnvironment(ns)
		devGitURL := "https://github.com/" + targetFullName
		devEnv.Spec.Source.URL = devGitURL

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
		}

		po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
		po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
		po.Namespace = ns
		po.Build = "1"
		po.Pipeline = "myorg/myapp/master"
		po.DevEnvContext.VersionResolver = jxtesthelpers.CreateTestVersionResolver(t)
		po.DevEnvContext.Requirements = &jxcore.RequirementsConfig{
			Environments: []jxcore.EnvironmentConfig{
				{
					Key:               "dev",
					Namespace:         "jx",
					PromotionStrategy: v1.PromotionStrategyTypeNever,
					GitURL:            devGitURL,
				},
				{
					Key:               "staging",
					Namespace:         "jx-staging",
					PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
				},
				{
					Key:               "production",
					Namespace:         "jx-production",
					PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
					GitURL:            tc.envSourceURL,
				},
			},
		}
		err := po.Run()
		require.NoError(t, err, "failed test %s s", name)

		scmClient := po.ScmClient
		require.NotNil(t, scmClient, "no ScmClient created")
		ctx := context.Background()

		for repoFullName, expectedCount := range tc.expectedPullRequestCount {
			prs, _, err := scmClient.PullRequests.List(ctx, repoFullName, scm.PullRequestListOptions{
				Size: 100,
				Open: true,
			})
			require.NoError(t, err, "failed to query PullRequests for repository %s test %s", repoFullName, name)
			require.Len(t, prs, expectedCount, "PullRequests for repository %s test %s", repoFullName, name)

			for _, pr := range prs {
				prNumber := pr.Number
				if pr.Link == "" {
					pr.Link = "https://github.com/" + repoFullName + "/pull/" + strconv.Itoa(prNumber)
				}
				t.Logf("%s created PullRequest %s #%d", name, pr.Link, prNumber)
				t.Logf("%s PR title: %s", name, pr.Title)
				t.Logf("%s PR body: %s", name, pr.Body)
			}
		}

		// lets assert we have a PipelineActivity...
		paList, err := po.JXClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
		require.NoError(t, err, "failed to load PipelineActivity resources in namespace %s", ns)
		require.Len(t, paList.Items, 1, "should have a PipelineActivity in namespace %s", ns)
		pa := paList.Items[0]

		data, err := yaml.Marshal(pa)
		require.NoError(t, err, "failed to marshal PipelineActivity")

		t.Logf("got PipelineActivity %s\n", string(data))
		assert.Equal(t, v1.ActivityStatusTypeSucceeded, pa.Spec.Status, "pipelineActivity.Spec.Status")
	}
}

func TestPromoteHelmfileRemoteCluster(t *testing.T) {
	version := "1.2.3"
	appName := "myapp"
	ns := "jx"

	runner := NewFakeRunnerWithGitClone()

	_, po := promote.NewCmdPromote()
	name := "promote-all"
	po.DisableGitConfig = true
	po.Application = appName
	po.Version = version
	po.All = true

	po.NoPoll = true
	po.BatchMode = true
	po.GitKind = "fake"
	po.CommandRunner = runner.Run
	po.AppGitURL = "https://github.com/myorg/myapp.git"

	targetFullName := "jenkins-x/default-environment-helmfile"

	devEnv := jxtesthelpers.CreateTestDevEnvironment(ns)
	devGitURL := "https://github.com/jenkins-x-labs-bdd-tests/jx3-kubernetes-jenkins"
	devEnv.Spec.Source.URL = devGitURL

	chartMuseumHost := "chartmuseum-jx.1.2.3.4.nip.io"
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
		&v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "chartmuseum",
				Namespace: ns,
			},
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host: chartMuseumHost,
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: "",
										Backend: v1beta1.IngressBackend{
											ServiceName: kube.ServiceChartMuseum,
											ServicePort: intstr.IntOrString{
												IntVal: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kube.ServiceChartMuseum,
				Namespace: ns,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "http",
						Protocol: "TCP",
						Port:     80,
					},
				},
			},
		},
	}
	jxObjects := []runtime.Object{
		devEnv,
	}

	po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
	po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
	po.Namespace = ns
	po.Build = "1"
	po.Pipeline = "myorg/myapp/master"
	po.DevEnvContext.VersionResolver = jxtesthelpers.CreateTestVersionResolver(t)
	po.DevEnvContext.Requirements = &jxcore.RequirementsConfig{
		Cluster: jxcore.ClusterConfig{
			DestinationConfig: jxcore.DestinationConfig{
				ChartRepository: "http://jenkins-x-chartmuseum.jx.svc.cluster.local:8080",
			},
		},
		Environments: []jxcore.EnvironmentConfig{
			{
				Key:               "dev",
				Namespace:         "jx",
				PromotionStrategy: v1.PromotionStrategyTypeNever,
				GitURL:            devGitURL,
			},
			{
				Key:               "production",
				Namespace:         "prodns",
				PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
				Owner:             "jstrachan",
				Repository:        "jx-demo-gke2-prod",
				RemoteCluster:     true,
			},
		},
	}

	err := po.Run()
	require.NoError(t, err, "failed test %s s", name)

	require.DirExists(t, po.OutDir, "should have an output dir from the generated PR")
	promotedHelmfile := filepath.Join(po.OutDir, "helmfiles", "prodns", "helmfile.yaml")
	require.FileExists(t, promotedHelmfile, "should have created the helmfile in the promote namespace")
	t.Logf("created promotion helmfile %s\n", promotedHelmfile)

	// lets verify we used the correct helm chart URL
	helmState := &state.HelmState{}
	err = yaml2s.LoadFile(promotedHelmfile, helmState)
	require.NoError(t, err, "failed to load helmfile %s", promotedHelmfile)

	require.Len(t, helmState.Repositories, 1, "should have 1 repository for %s", promotedHelmfile)
	helmRepo := helmState.Repositories[0]
	t.Logf("promoted with helm repository %s = %s in file %s\n", helmRepo.Name, helmRepo.URL, promotedHelmfile)

	assert.Equal(t, "dev", helmRepo.Name, "repositories[0].name for %s", promotedHelmfile)
	assert.Equal(t, "http://"+chartMuseumHost, helmRepo.URL, "repositories[0].url for %s", promotedHelmfile)

	scmClient := po.ScmClient
	require.NotNil(t, scmClient, "no ScmClient created")
	ctx := context.Background()

	prNumber := 1
	pr, _, err := scmClient.PullRequests.Find(ctx, targetFullName, prNumber)
	require.NoError(t, err, "failed to find repository %s number %d", targetFullName, prNumber)
	assert.NotNil(t, pr, "nil pr %s for %s", targetFullName, prNumber)

	t.Logf("created PullRequest %s #%d", pr.Link, prNumber)
	t.Logf("PR title: %s", pr.Title)
	t.Logf("PR body: %s", pr.Body)

	// lets assert we have a PipelineActivity...
	paList, err := po.JXClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err, "failed to load PipelineActivity resources in namespace %s", ns)
	require.Len(t, paList.Items, 1, "should have a PipelineActivity in namespace %s", ns)
	pa := paList.Items[0]

	data, err := yaml.Marshal(pa)
	require.NoError(t, err, "failed to marshal PipelineActivity")

	t.Logf("got PipelineActivity %s\n", string(data))
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, pa.Spec.Status, "pipelineActivity.Spec.Status")

}

func TestPromoteHelmfileRemoteClusterWithOCI(t *testing.T) {
	version := "1.2.3"
	appName := "myapp"
	ns := "jx"

	runner := NewFakeRunnerWithGitClone()

	_, po := promote.NewCmdPromote()
	name := "promote-all"
	po.DisableGitConfig = true
	po.Application = appName
	po.Version = version
	po.All = true

	po.NoPoll = true
	po.BatchMode = true
	po.GitKind = "fake"
	po.CommandRunner = runner.Run
	po.AppGitURL = "https://github.com/myorg/myapp.git"

	targetFullName := "jenkins-x/default-environment-helmfile"

	devEnv := jxtesthelpers.CreateTestDevEnvironment(ns)
	devGitURL := "https://github.com/jenkins-x-labs-bdd-tests/jx3-kubernetes-jenkins"
	devEnv.Spec.Source.URL = devGitURL

	chartRepository := "harbor.jx.cheese/myowner"

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
	}

	po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
	po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
	po.Namespace = ns
	po.Build = "1"
	po.Pipeline = "myorg/myapp/master"
	po.DevEnvContext.VersionResolver = jxtesthelpers.CreateTestVersionResolver(t)
	po.DevEnvContext.Requirements = &jxcore.RequirementsConfig{
		Cluster: jxcore.ClusterConfig{
			DestinationConfig: jxcore.DestinationConfig{
				ChartRepository: chartRepository,
				ChartKind:       jxcore.ChartRepositoryTypeOCI,
			},
		},
		Environments: []jxcore.EnvironmentConfig{
			{
				Key:               "dev",
				Namespace:         "jx",
				PromotionStrategy: v1.PromotionStrategyTypeNever,
				GitURL:            devGitURL,
			},
			{
				Key:               "production",
				Namespace:         "prodns",
				PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
				Owner:             "jstrachan",
				Repository:        "jx-demo-gke2-prod",
				RemoteCluster:     true,
			},
		},
	}

	err := po.Run()
	require.NoError(t, err, "failed test %s s", name)

	require.DirExists(t, po.OutDir, "should have an output dir from the generated PR")
	promotedHelmfile := filepath.Join(po.OutDir, "helmfiles", "prodns", "helmfile.yaml")
	require.FileExists(t, promotedHelmfile, "should have created the helmfile in the promote namespace")
	t.Logf("created promotion helmfile %s\n", promotedHelmfile)

	// lets verify we used the correct helm chart URL
	helmState := &state.HelmState{}
	err = yaml2s.LoadFile(promotedHelmfile, helmState)
	require.NoError(t, err, "failed to load helmfile %s", promotedHelmfile)

	require.Len(t, helmState.Repositories, 1, "should have 1 repository for %s", promotedHelmfile)
	helmRepo := helmState.Repositories[0]
	t.Logf("promoted with helm repository %s = %s in file %s\n", helmRepo.Name, helmRepo.URL, promotedHelmfile)

	assert.Equal(t, "dev", helmRepo.Name, "repositories[0].name for %s", promotedHelmfile)
	assert.Equal(t, chartRepository, helmRepo.URL, "repositories[0].url for %s", promotedHelmfile)
	assert.Equal(t, true, helmRepo.OCI, "repositories[0].oci for %s", promotedHelmfile)

	scmClient := po.ScmClient
	require.NotNil(t, scmClient, "no ScmClient created")
	ctx := context.Background()

	prNumber := 1
	pr, _, err := scmClient.PullRequests.Find(ctx, targetFullName, prNumber)
	require.NoError(t, err, "failed to find repository %s number %d", targetFullName, prNumber)
	assert.NotNil(t, pr, "nil pr %s for %s", targetFullName, prNumber)

	t.Logf("created PullRequest %s #%d", pr.Link, prNumber)
	t.Logf("PR title: %s", pr.Title)
	t.Logf("PR body: %s", pr.Body)

	// lets assert we have a PipelineActivity...
	paList, err := po.JXClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err, "failed to load PipelineActivity resources in namespace %s", ns)
	require.Len(t, paList.Items, 1, "should have a PipelineActivity in namespace %s", ns)
	pa := paList.Items[0]

	data, err := yaml.Marshal(pa)
	require.NoError(t, err, "failed to marshal PipelineActivity")

	t.Logf("got PipelineActivity %s\n", string(data))
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, pa.Spec.Status, "pipelineActivity.Spec.Status")

}

func TestPromoteHelmfileToNamedLocalEnvironment(t *testing.T) {
	version := "1.2.3"
	appName := "myapp"
	ns := "jx"

	runner := NewFakeRunnerWithGitClone()

	_, po := promote.NewCmdPromote()
	name := "promote-local"
	po.DisableGitConfig = true
	po.Application = appName
	po.Version = version
	po.Environment = "staging"

	po.NoPoll = true
	po.BatchMode = true
	po.GitKind = "fake"
	po.CommandRunner = runner.Run
	po.AppGitURL = "https://github.com/myorg/myapp.git"

	targetFullName := "jenkins-x/default-environment-helmfile"

	devEnv := jxtesthelpers.CreateTestDevEnvironment(ns)
	devGitURL := "https://github.com/jenkins-x-labs-bdd-tests/jx3-kubernetes-jenkins"
	devEnv.Spec.Source.URL = devGitURL

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
	}

	po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
	po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
	po.Namespace = ns
	po.Build = "1"
	po.Pipeline = "myorg/myapp/master"
	po.DevEnvContext.VersionResolver = jxtesthelpers.CreateTestVersionResolver(t)
	po.DevEnvContext.Requirements = &jxcore.RequirementsConfig{
		Environments: []jxcore.EnvironmentConfig{
			{
				Key:               "dev",
				Namespace:         "jx",
				PromotionStrategy: v1.PromotionStrategyTypeNever,
				GitURL:            devGitURL,
			},
			{
				Key:       "staging",
				Namespace: "jx-staging",
			},
			{
				Key:               "production",
				Namespace:         "jx-production",
				PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
				Repository:        "jx-production",
				RemoteCluster:     true,
			},
		},
	}

	err := po.Run()
	require.NoError(t, err, "failed test %s s", name)

	scmClient := po.ScmClient
	require.NotNil(t, scmClient, "no ScmClient created")
	ctx := context.Background()

	prNumber := 1
	pr, _, err := scmClient.PullRequests.Find(ctx, targetFullName, prNumber)
	require.NoError(t, err, "failed to find repository %s number %d", targetFullName, prNumber)
	assert.NotNil(t, pr, "nil pr %s for %s", targetFullName, prNumber)

	t.Logf("created PullRequest %s #%d", pr.Link, prNumber)
	t.Logf("PR title: %s", pr.Title)
	t.Logf("PR body: %s", pr.Body)
	var labels []string
	for _, l := range pr.Labels {
		t.Logf("PR label: %s", l.Name)
		labels = append(labels, l.Name)
	}
	assert.Contains(t, labels, "env/staging", "should have label")
	assert.NotContains(t, labels, "do-not-merge/hold", "should not have label")

	// lets assert we have a PipelineActivity...
	paList, err := po.JXClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err, "failed to load PipelineActivity resources in namespace %s", ns)
	require.Len(t, paList.Items, 1, "should have a PipelineActivity in namespace %s", ns)
	pa := paList.Items[0]

	data, err := yaml.Marshal(pa)
	require.NoError(t, err, "failed to marshal PipelineActivity")

	t.Logf("got PipelineActivity %s\n", string(data))
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, pa.Spec.Status, "pipelineActivity.Spec.Status")
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

		devEnv := jxtesthelpers.CreateTestDevEnvironment(ns)

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
		}
		po.DevEnvContext.Requirements = &jxcore.RequirementsConfig{
			Environments: []jxcore.EnvironmentConfig{
				{
					Key:               envName,
					Namespace:         "jx-" + envName,
					PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
					GitURL:            tc.gitURL,
				},
			},
		}
		po.DevEnvContext.VersionResolver = jxtesthelpers.CreateTestVersionResolver(t)

		po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
		po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
		po.Namespace = ns
		po.Build = "1"
		po.Pipeline = "myorg/myapp/master"

		err := po.Run()
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
		paList, err := po.JXClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
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
