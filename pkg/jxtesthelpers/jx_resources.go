package jxtesthelpers

import (
	"path"
	"testing"

	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"
	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jxenv"
	"github.com/jenkins-x/jx-helpers/v3/pkg/versionstream"
	"github.com/jenkins-x/jx-promote/pkg/envctx"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
)

func CreateTestDevEnvironment(ns string) (*v1.Environment, error) {
	devEnv := jxenv.CreateDefaultDevEnvironment(ns)
	devEnv.Namespace = ns

	// lets add a requirements object
	req := CreateTestRequirements()
	data, err := yaml.Marshal(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal requirements %#v to YAML", req)
	}
	devEnv.Spec.TeamSettings.BootRequirements = string(data)
	return devEnv, err
}

func CreateTestRequirements() *jxcore.RequirementsConfig {
	req := jxcore.NewRequirementsConfig()
	return &req.Spec
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
	requirementsConfig := CreateTestRequirements()

	devEnv, err := CreateTestDevEnvironment(ns)
	require.NoError(t, err, "failed to create test dev Environemnt")

	return &envctx.EnvironmentContext{
		GitOps:          true,
		Requirements:    requirementsConfig,
		DevEnv:          devEnv,
		VersionResolver: vr,
	}
}
