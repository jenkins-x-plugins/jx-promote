module github.com/jenkins-x/jx-promote

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/jenkins-x/go-scm v1.5.178
	github.com/jenkins-x/jx-api/v3 v3.0.0
	github.com/jenkins-x/jx-gitops v0.0.372
	github.com/jenkins-x/jx-helpers/v3 v3.0.0
	github.com/jenkins-x/jx-logging/v3 v3.0.0
	github.com/pkg/errors v0.9.1
	github.com/roboll/helmfile v0.125.7
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/helm v2.16.12+incompatible
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/jenkins-x/lighthouse => github.com/rawlingsj/lighthouse v0.0.0-20201005083317-4d21277f7992
	k8s.io/client-go => k8s.io/client-go v0.19.2
)

go 1.15
