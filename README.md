# jx-promote

[![Documentation](https://godoc.org/github.com/jenkins-x/jx-promote?status.svg)](https://pkg.go.dev/mod/github.com/jenkins-x/jx-promote)
[![Go Report Card](https://goreportcard.com/badge/github.com/jenkins-x/jx-promote)](https://goreportcard.com/report/github.com/jenkins-x/jx-promote)
[![Releases](https://img.shields.io/github/release-pre/jenkins-x-labs/helmboot.svg)](https://github.com/jenkins-x/jx-promote/releases)
[![LICENSE](https://img.shields.io/github/license/jenkins-x-labs/helmboot.svg)](https://github.com/jenkins-x/jx-promote/blob/master/LICENSE)
[![Slack Status](https://img.shields.io/badge/slack-join_chat-white.svg?logo=slack&style=social)](https://slack.k8s.io/)

`jx promote` is an experimental binary plugin to promote applications to a [Jenkins](https://jenkins.io/) environment

## Getting Started

Download the [jx-alpha-promote binary](https://github.com/jenkins-x/jx-promote/releases) for your operating system and add it to your `$PATH`.

There will be an `app` you can install soon too...

## Promoting via the command line

Just run the `jx alpha promote` command line and follow the instructions as if it were `jx promote`.

## Rules

`jx promote` supports a number of different rules for promoting new versions of applications for various kinds of deployment tools.

### Helm

The helm rule uses a helm chart's `requirements.yaml` file to manage dependent applications. This is the traditional default source layout of a Jenkins X Staging or Production repository; there is usually a helm chart in the `env` folder and `jx-promote` will create a Pull Request adding or updating the applications version in `env/requirements.yaml`.

You can [explicitly configure]() the helm rule by specifying the `helmRule` property on the [spec](https://github.com/jenkins-x/jx-promote/blob/master/docs/config.md#promote.jenkins-x.io/v1alpha1.PromoteSpec) of the [.jx/promote.yaml](https://github.com/jenkins-x/jx-promote/blob/master/docs/config.md#promote) configuration file. 


## Rule Configuration

You can configure which promotion rule is used declaratively in your environment git repository by creating a [.jx/promote.yaml](https://github.com/jenkins-x/jx-promote/blob/master/docs/config.md#promote) configuration file. 

 
 

