package helmfile_test

import (
	"testing"

	"github.com/jenkins-x-plugins/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x-plugins/jx-promote/pkg/envctx"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/helmfile"
	"github.com/stretchr/testify/assert"
)

func TestRule(t *testing.T) {
	pr := &rules.PromoteRule{
		TemplateContext: rules.TemplateContext{},
		Dir:             "",
		Config:          v1alpha1.Promote{},
		DevEnvContext:   &envctx.EnvironmentContext{},
	}
	err := helmfile.Rule(pr)
	assert.Error(t, err)
}
