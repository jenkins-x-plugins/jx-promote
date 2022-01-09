package jxtesthelpers

import (
	"path"
	"testing"

	"github.com/jenkins-x-plugins/jx-promote/pkg/envctx"
	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jxenv"
	"github.com/jenkins-x/jx-helpers/v3/pkg/versionstream"
	"github.com/stretchr/testify/assert"
)

func CreateTestDevEnvironment(ns string) *v1.Environment {
	devEnv := jxenv.CreateDefaultDevEnvironment(ns)
	devEnv.Namespace = ns

	// lets add a requirements object
	devEnv.Spec.Source.URL = "https://github.com/jx3-gitops-repositories/jx3-kubernetes.git"
	return devEnv
}

func CreateTestRequirements() *jxcore.RequirementsConfig {
	req := jxcore.NewRequirementsConfig()
	return &req.Spec
}

func CreateTestVersionResolver(t *testing.T) *versionstream.VersionResolver {
	versionsDir := path.Join("testdata", "jenkins-x-versions")
	assert.DirExists(t, versionsDir)

	return &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}
}

func CreateTestDevEnvironmentContext(t *testing.T, ns string) *envctx.EnvironmentContext {
	vr := CreateTestVersionResolver(t)
	requirementsConfig := CreateTestRequirements()

	devEnv := CreateTestDevEnvironment(ns)

	return &envctx.EnvironmentContext{
		GitOps:          true,
		Requirements:    requirementsConfig,
		DevEnv:          devEnv,
		VersionResolver: vr,
	}
}
