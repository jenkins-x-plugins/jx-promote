package environments

import (
	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-api/v3/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/helmer"
	"github.com/jenkins-x/jx-helpers/v3/pkg/scmhelpers"
	"github.com/jenkins-x/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x/jx-promote/pkg/envctx"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

//ValuesFiles is a wrapper for a slice of values files to allow them to be passed around as a pointer
type ValuesFiles struct {
	Items []string
}

// ModifyChartFn callback for modifying a chart, requirements, the chart metadata,
// the values.yaml and all files in templates are unmarshaled, and the root dir for the chart is passed
type ModifyChartFn func(requirements *helmer.Requirements, metadata *chart.Metadata, existingValues map[string]interface{},
	templates map[string]string, dir string, pullRequestDetails *scm.PullRequest) error

// ModifyKptFn callback for modifying the kpt based installations of resources
type ModifyKptFn func(dir string, promoteConfig *v1alpha1.Promote, pullRequestDetails *scm.PullRequest) error

// EnvironmentPullRequestOptions are options for creating a pull request against an environment.
// The provide a Gitter client for performing git operations, a GitProvider client for talking to the git provider,
// a callback ModifyChartFn which is where the changes you want to make are defined,
type EnvironmentPullRequestOptions struct {
	// PullRequestFilter used to find an existing Pull Request to rebase/modify
	PullRequestFilter *PullRequestFilter

	DevEnvContext    envctx.EnvironmentContext
	ScmClientFactory scmhelpers.Factory
	Gitter           gitclient.Interface
	CommandRunner    cmdrunner.CommandRunner

	Function            func() error
	ModifyChartFn       ModifyChartFn
	ModifyKptFn         ModifyKptFn
	PullRequestNumber   int
	Labels              []string
	GitKind             string
	OutDir              string
	RemoteName          string
	BaseBranchName      string
	BranchName          string
	CommitTitle         string
	CommitMessage       string
	CommitMessageSuffix string
	Namespace           string
	JXClient            versioned.Interface
	ScmClient           *scm.Client
	BatchMode           bool
	UseGitHubOAuth      bool
	Fork                bool
}

// A PullRequestFilter defines a filter for finding pull requests
type PullRequestFilter struct {
	Labels []string
	Number *int
}
