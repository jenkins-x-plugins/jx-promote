module github.com/jenkins-x/jx-promote

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/cli/cli v0.6.2
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.3.1
	github.com/google/uuid v1.1.1
	github.com/jenkins-x/go-scm v1.5.141
	github.com/jenkins-x/jx-kube-client v0.0.3
	github.com/jenkins-x/jx-logging v0.0.8
	github.com/jenkins-x/jx/v2 v2.1.78
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.7.0
	github.com/onsi/gomega v1.4.3
	github.com/petergtz/pegomock v2.7.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/satori/go.uuid v1.2.1-0.20180103174451-36e9d2ebbde5
	github.com/spf13/cobra v0.0.6
	github.com/stretchr/testify v1.6.0
	gopkg.in/AlecAivazis/survey.v1 v1.8.3
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c
	k8s.io/api v0.0.0-20190816222004-e3a6b8045b0b
	k8s.io/apimachinery v0.0.0-20190816221834-a9f1d8a9c101
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/helm v2.7.2+incompatible
	k8s.io/kubernetes v1.11.3
	sigs.k8s.io/yaml v1.1.0

)

replace github.com/heptio/sonobuoy => github.com/jenkins-x/sonobuoy v0.11.7-0.20190318120422-253758214767

replace k8s.io/api => k8s.io/api v0.0.0-20190528110122-9ad12a4af326

replace k8s.io/metrics => k8s.io/metrics v0.0.0-20181128195641-3954d62a524d

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221084156-01f179d85dbc

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190528110200-4f3abb12cae2

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190528110544-fa58353d80f3

replace github.com/sirupsen/logrus => github.com/jtnord/logrus v1.4.2-0.20190423161236-606ffcaf8f5d

replace github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v21.1.0+incompatible

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v10.15.5+incompatible

replace github.com/banzaicloud/bank-vaults => github.com/banzaicloud/bank-vaults v0.0.0-20190508130850-5673d28c46bd

replace github.com/TV4/logrus-stackdriver-formatter => github.com/jenkins-x/logrus-stackdriver-formatter v0.1.1-0.20200408213659-1dcf20c371bb

replace code.gitea.io/sdk/gitea => code.gitea.io/sdk/gitea v0.12.0

go 1.13
