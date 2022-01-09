# jx-promote

[![Documentation](https://godoc.org/github.com/jenkins-x-plugins/jx-promote?status.svg)](https://pkg.go.dev/mod/github.com/jenkins-x-plugins/jx-promote)
[![Go Report Card](https://goreportcard.com/badge/github.com/jenkins-x-plugins/jx-promote)](https://goreportcard.com/report/github.com/jenkins-x-plugins/jx-promote)
[![Releases](https://img.shields.io/github/release-pre/jenkins-x/helmboot.svg)](https://github.com/jenkins-x-plugins/jx-promote/releases)
[![LICENSE](https://img.shields.io/github/license/jenkins-x/helmboot.svg)](https://github.com/jenkins-x-plugins/jx-promote/blob/master/LICENSE)
[![Slack Status](https://img.shields.io/badge/slack-join_chat-white.svg?logo=slack&style=social)](https://slack.k8s.io/)

`jx promote` is a binary plugin to promote applications to one or more [Jenkins X](https://jenkins-x.io/) environments

## Getting Started

Download the [jx-promote binary](https://github.com/jenkins-x-plugins/jx-promote/releases) for your operating system and add it to your `$PATH`.

## Commands

See the [jx-promote command reference](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/cmd/jx-promote.md#jx-promote)

## Promoting via the command line

Just run the `jx promote` command line and follow the instructions as if it were `jx promote`.

## Rules

`jx promote` supports a number of different rules for promoting new versions of applications for various kinds of deployment tools.

### Helm

The helm rule uses a helm chart's `requirements.yaml` file to manage dependent applications. This is the  default source layout of a Jenkins X Staging or Production repository; there is usually a helm chart in the `env` folder and `jx-promote` will create a Pull Request adding or updating the applications version in `env/requirements.yaml`.


`jx promote` will detect the `env/requirements.yaml` file automatically without any explict configuration.

You can [explicitly configure](#rule-configuration) the helm rule by specifying the [helmRule](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#promote.jenkins-x.io/v1alpha1.HelmRule) property on the [spec](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#promote.jenkins-x.io/v1alpha1.PromoteSpec) of the [.jx/promote.yaml](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#promote) configuration file like [this one](pkg/rules/factory/testdata/helm-explicit/.jx/promote.yaml#L4-L5):

```yaml 
apiVersion: promote.jenkins-x.io/v1alpha1
kind: Promote
spec:
  helmRule:
    path: env
```


### Apps

The apps rule uses a `jx-apps.yml` file to describe the charts to deploy in your environments git repository.
 
`jx promote` will detect the `jx-apps.yml` file in the root directory automatically without any explicit configuration.


You can [explicitly configure](#rule-configuration) the apps rule by creating a [.jx/promote.yaml](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#promote) configuration file and specifying the [appsRule](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#appsrule) like in [this one](pkg/rules/factory/testdata/jx-apps-explicit/.jx/promote.yaml#L4-L5)

```yaml 
apiVersion: promote.jenkins-x.io/v1alpha1
kind: Promote
spec:
  appsRule:
    path: jx-apps.yml
```


### Helmfile

The helmfile rule uses a `helmfile.yaml` file from [helmfile](https://github.com/roboll/helmfile) to configure the helm charts to deploy to your environment.
            
`jx promote` will detect the `helmfile.yaml` file in the root directory automatically without any explicit configuration.

You can [explicitly configure](#rule-configuration) the helmfile rule by creating a [.jx/promote.yaml](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#promote) configuration file and specifying the [helmfile rule](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#helmfilerule) like [this one](pkg/rules/factory/testdata/helmfile-explicit/.jx/promote.yaml#L4-L5):

```yaml 
apiVersion: promote.jenkins-x.io/v1alpha1
kind: Promote
spec:
  helmfileRule:
    path: helmfile.yaml
``` 

### File

The file rule can modify arbitrary files such as `Makefile` or shell scripts to include a promotion command using tools like [helm](https://helm.sh/) or [kpt](https://googlecontainertools.github.io/kpt/)

To enable the file mode you need to create a [.jx/promote.yaml](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#promote) configuration file and specifying the [file rule](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#filerule).

For example to promote into a `Makefile` using `helm template` you could create a file like [this one](pkg/rules/factory/testdata/make-helm/.jx/promote.yaml#L4-L12):
                                           
```yaml 
apiVersion: promote.jenkins-x.io/v1alpha1
kind: Promote
spec:
  fileRule:
    path: Makefile
    linePrefix: "\t"
    insertAfter:
    - prefix: "helm template"
    - prefix: "fetch:"
    updateTemplate:
      regex: "helm template --namespace {{.Namespace}} --version .* {{.AppName}} .*"
    commandTemplate: "helm template --namespace {{.Namespace}} --version {{.Version}} {{.AppName}} dev/{{.AppName}}"
``` 

Or to use [kpt](https://googlecontainertools.github.io/kpt/) to promote you could use [this one](pkg/rules/factory/testdata/make-kpt/.jx/promote.yaml#L4-L12):
                                           
```yaml 
apiVersion: promote.jenkins-x.io/v1alpha1
kind: Promote
spec:
  fileRule:
    path: Makefile
    linePrefix: "\t"
    insertAfter:
    - prefix: "kpt pkg get"
    - prefix: "fetch:"
    updateTemplate:
      prefix: "kpt pkg get {{.GitURL}}"
    commandTemplate: "kpt pkg get {{.GitURL}}/kubernetes@v{{.Version}} $(FETCH_DIR)/namespaces/jx"
``` 

if you are using a script to in your environment git repository you could use a configuration like  [this one](pkg/rules/factory/testdata/script-kpt/.jx/promote.yaml#L4-L12):

```yaml 
apiVersion: promote.jenkins-x.io/v1alpha1
kind: Promote
spec:
  fileRule:
    path: myscript.sh
    insertAfter:
    - prefix: "kpt pkg get"
    - prefix: "# fetch resources"
    updateTemplate:
      prefix: "kpt pkg get {{.GitURL}}"
    commandTemplate: "kpt pkg get {{.GitURL}}/kubernetes@v{{.Version}} $(FETCH_DIR)/namespaces/jx"
```                                                                                                           

## Rule Configuration

`jx promote` can automatically detect common configurations as described above or you can explicilty configure the promotion rule in your environment git repository by creating a [.jx/promote.yaml](https://github.com/jenkins-x-plugins/jx-promote/blob/master/docs/config.md#promote) configuration file. 

For example if you wish to configure the [helm rule](#helm) you may want to use a `.jx/promote.yaml` file like [this one](pkg/rules/factory/testdata/helm-explicit/.jx/promote.yaml#L4-L5):

```yaml 
apiVersion: promote.jenkins-x.io/v1alpha1
kind: Promote
spec:
  helmRule:
    path: env
```


 
 

