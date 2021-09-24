// +build unit

package promote_test

import (
	"sort"
	"testing"

	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	"github.com/stretchr/testify/require"

	"github.com/jenkins-x-plugins/jx-promote/pkg/promote"
	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-helpers/v3/pkg/input/fake"
	"github.com/jenkins-x/jx-helpers/v3/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
)

func fakeSearchForChart(f string) (string, error) {
	return "mySearchedApp", nil
}

func fakeDiscoverAppName() (string, error) {
	return "myDiscoveredApp", nil
}

func fakeChooseChart() (string, error) {
	return "myChosenApp", nil
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlagWithArgs(t *testing.T) {
	promoteOptions := &promote.Options{
		Environment: "production", // --env production
	}

	promoteOptions.Args = []string{"myArgumentApp"}

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName, fakeChooseChart)
	assert.NoError(t, err)

	assert.Equal(t, "myArgumentApp", promoteOptions.Application)
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlagWithFilterFlag(t *testing.T) {
	promoteOptions := &promote.Options{
		Environment: "production", // --env production
		Filter:      "something",
	}

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName, fakeChooseChart)
	assert.NoError(t, err)

	assert.Equal(t, "mySearchedApp", promoteOptions.Application)
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlagWithBatchFlag(t *testing.T) {
	promoteOptions := &promote.Options{
		Environment: "production", // --env production
	}

	promoteOptions.BatchMode = true // --batch-mode

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName, fakeChooseChart)
	assert.NoError(t, err)

	assert.Equal(t, "myDiscoveredApp", promoteOptions.Application)
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlagWithInteractiveFlag(t *testing.T) {
	promoteOptions := &promote.Options{
		Environment: "production", // --env production
		Interactive: true,
	}

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName, fakeChooseChart)
	assert.NoError(t, err)

	assert.Equal(t, "myChosenApp", promoteOptions.Application)
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlag(t *testing.T) {
	testhelpers.SkipForWindows(t, "go-expect does not work on windows")

	promoteOptions := &promote.Options{
		Environment: "production", // --env production
	}

	promoteOptions.Input = &fake.FakeInput{
		Values: map[string]string{"Are you sure you want to promote the application named: myDiscoveredApp?": "Y"},
	}

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName, fakeChooseChart)

	assert.NoError(t, err)
	assert.Equal(t, "myDiscoveredApp", promoteOptions.Application)
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlagUserSaysNo(t *testing.T) {
	testhelpers.SkipForWindows(t, "go-expect does not work on windows")

	promoteOptions := &promote.Options{
		Environment: "production", // --env production
		Input: &fake.FakeInput{
			Values: map[string]string{"Are you sure you want to promote the application named: myDiscoveredApp?": "N"},
		},
	}

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName, fakeChooseChart)
	assert.Error(t, err)
	assert.Equal(t, "", promoteOptions.Application)
}

func TestGetEnvChartValues(t *testing.T) {
	tests := []struct {
		ns           string
		env          jxcore.EnvironmentConfig
		values       []string
		valueStrings []string
	}{{
		"jx-custom-env",
		jxcore.EnvironmentConfig{
			Key:               "custom-env",
			Namespace:         "jx-custom-env",
			PromotionStrategy: v1.PromotionStrategyTypeManual,
			GitURL:            "https://github.com/my-project/jx-environment-custom-env",
		},
		[]string{
			"tags.jx-env-custom-env=true",
			"tags.jx-ns-jx-custom-env=true",
			"global.jxEnvCustomEnv=true",
			"global.jxNsJxCustomEnv=true",
		},
		[]string{
			"global.jxEnv=custom-env",
			"global.jxNs=jx-custom-env",
		},
	}, {
		"ns-rand",
		jxcore.EnvironmentConfig{
			Key:               "random-env",
			Namespace:         "ns-other",
			PromotionStrategy: v1.PromotionStrategyTypeNever,
			GitURL:            "https://github.com/my-project/random",
		},
		[]string{
			"tags.jx-env-random-env=true",
			"tags.jx-ns-ns-rand=true",
			"global.jxEnvRandomEnv=true",
			"global.jxNsNsRand=true",
		},
		[]string{
			"global.jxEnv=random-env",
			"global.jxNs=ns-rand",
		},
	}}

	for _, test := range tests {
		promoteOptions := &promote.Options{}
		values, valueStrings := promoteOptions.GetEnvChartValues(test.ns, &test.env)
		sort.Strings(values)
		sort.Strings(test.values)
		assert.Equal(t, values, test.values)
		sort.Strings(valueStrings)
		sort.Strings(test.valueStrings)
		assert.Equal(t, valueStrings, test.valueStrings)
	}
}

func TestConvertToGitHubPagesURL(t *testing.T) {
	source := "https://github.com/cdfoundation/tekton-helm-chart"
	actual, err := promote.ConvertToGitHubPagesURL(source)
	require.NoError(t, err, "failed to parse source %s", source)
	assert.Equal(t, "https://cdfoundation.github.io/tekton-helm-chart/", actual, "for source %s", source)

	source = "https://something.com/cheese/wine"
	actual, err = promote.ConvertToGitHubPagesURL(source)
	require.Error(t, err, "should fail to convert to github pages URL %s", source)
	t.Logf("got expected failure %s for %s\n", err.Error(), source)
}

func TestIsLocalChartRepository(t *testing.T) {
	localRepos := []string{"http://jenkins-x-chartmuseum:8080", "http://jenkins-x-chartmuseum", "https://chartmuseum", "http://jenkins-x-chartmuseum.jx.svc.cluster.local:8080", "http://bucketrepo.jx"}
	for _, repo := range localRepos {
		actual := promote.IsLocalChartRepository(repo)
		assert.True(t, actual, "should be local repo %s", repo)
	}

	remoteRepos := []string{"http://foo.bar", "https://chartrepo.mydomain.com"}
	for _, repo := range remoteRepos {
		actual := promote.IsLocalChartRepository(repo)
		assert.False(t, actual, "not local repo %s", repo)
	}
}
