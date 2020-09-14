package rules

import (
	"github.com/jenkins-x/jx-helpers/pkg/cmdrunner"
	"github.com/jenkins-x/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x/jx-promote/pkg/envctx"
)

// PromoteRule represents a profile rule
type PromoteRule struct {
	TemplateContext
	Dir           string
	Config        v1alpha1.Promote
	DevEnvContext *envctx.EnvironmentContext
	CommandRunner cmdrunner.CommandRunner
}

// TemplateContext expressions used in templates
type TemplateContext struct {
	GitURL            string
	Version           string
	AppName           string
	ChartAlias        string
	Namespace         string
	HelmRepositoryURL string
}

// RuleFunction a rule function for evaluating the rule
type RuleFunction func(*PromoteRule) error
