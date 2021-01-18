module github.com/jenkins-x/jx-promote

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/hashicorp/vault/api v1.0.5-0.20190909201928-35325e2c3262 // indirect
	github.com/jenkins-x/go-scm v1.5.211
	github.com/jenkins-x/jx-api/v4 v4.0.22
	github.com/jenkins-x/jx-gitops v0.0.529
	github.com/jenkins-x/jx-helpers/v3 v3.0.63
	github.com/jenkins-x/jx-logging/v3 v3.0.3
	github.com/pkg/errors v0.9.1
	github.com/roboll/helmfile v0.135.1-0.20201213020320-54eb73b4239a
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	helm.sh/helm/v3 v3.5.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/jenkins-x/lighthouse => github.com/jstrachan/lighthouse v0.0.0-20210117125431-b895e82778b3

	k8s.io/api => k8s.io/api v0.20.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.2
	k8s.io/client-go => k8s.io/client-go v0.20.2
)

go 1.15
