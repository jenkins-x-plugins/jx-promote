module github.com/jenkins-x/jx-promote

require (
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.2 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/hashicorp/go-retryablehttp v0.6.6 // indirect
	github.com/hashicorp/vault/api v1.0.5-0.20190909201928-35325e2c3262 // indirect
	github.com/jenkins-x/go-scm v1.5.216
	github.com/jenkins-x/jx-api/v4 v4.0.23
	github.com/jenkins-x/jx-gitops v0.0.547
	github.com/jenkins-x/jx-helpers/v3 v3.0.73
	github.com/jenkins-x/jx-logging/v3 v3.0.3
	github.com/mitchellh/mapstructure v1.3.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/roboll/helmfile v0.138.2
	github.com/smartystreets/assertions v1.0.0 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	gopkg.in/ini.v1 v1.56.0 // indirect
	helm.sh/helm/v3 v3.5.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	sigs.k8s.io/yaml v1.2.0
)

replace (
	k8s.io/api => k8s.io/api v0.20.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.2
	k8s.io/client-go => k8s.io/client-go v0.20.2
)

go 1.15
