package factory

import (
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/file"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/helm"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/helmfile"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/kpt"
)

// NewFunction creates a function based on the kind of rule
func NewFunction(r *rules.PromoteRule) rules.RuleFunction {
	spec := r.Config.Spec
	if spec.FileRule != nil {
		return file.Rule
	}
	if spec.HelmRule != nil {
		return helm.Rule
	}
	if spec.HelmfileRule != nil {
		return helmfile.Rule
	}
	if spec.KptRule != nil {
		return kpt.Rule
	}
	return nil
}
