package rules

import "github.com/jenkins-x/jx-promote/pkg/apis/boot/v1alpha1"

// PromoteRule represents a profile rule
type PromoteRule struct {
	Dir     string
	Config  v1alpha1.Promote
	GitURL  string
	Version string
	AppName string
}

// TemplateContext expressions used in templates
type TemplateContext struct {
	GitURL  string
	Version string
	AppName string
}
