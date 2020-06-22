package testhelpers

import (
	"path"
	"testing"

	"github.com/jenkins-x/jx-promote/pkg/envctx"
	v1 "github.com/jenkins-x/jx/v2/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/v2/pkg/config"
	"github.com/jenkins-x/jx/v2/pkg/kube"
	"github.com/jenkins-x/jx/v2/pkg/versionstream"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
)

func CreateTestDevEnvironment(ns string) (*v1.Environment, error) {
	devEnv := kube.CreateDefaultDevEnvironment(ns)
	devEnv.Namespace = ns

	// lets add a requirements object
	req := CreateTestRequirements(ns)
	data, err := yaml.Marshal(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal requirements %#v to YAML", req)
	}
	devEnv.Spec.TeamSettings.BootRequirements = string(data)
	return devEnv, err
}

func CreateTestRequirements(ns string) *config.RequirementsConfig {
	req := config.NewRequirementsConfig()
	req.Cluster.Namespace = ns
	return req
}

func CreateTestVersionResolver(t *testing.T) *versionstream.VersionResolver {
	versionsDir := path.Join("test_data", "jenkins-x-versions")
	assert.DirExists(t, versionsDir)

	return &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}
}

func CreateTestDevEnvironmentContext(t *testing.T, ns string) *envctx.EnvironmentContext {
	vr := CreateTestVersionResolver(t)
	requirementsConfig := CreateTestRequirements(ns)

	devEnv, err := CreateTestDevEnvironment(ns)
	require.NoError(t, err, "failed to create test dev Environemnt")

	return &envctx.EnvironmentContext{
		GitOps:          true,
		Requirements:    requirementsConfig,
		DevEnv:          devEnv,
		VersionResolver: vr,
	}
}
