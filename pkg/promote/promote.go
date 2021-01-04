package promote

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx-helpers/v3/pkg/requirements"

	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/cli"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-gitops/pkg/cmd/git/setup"
	"github.com/jenkins-x/jx-helpers/v3/pkg/builds"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/gitdiscovery"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/v3/pkg/input"
	"github.com/jenkins-x/jx-helpers/v3/pkg/input/survey"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/activities"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jxclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jxenv"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/services"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-helpers/v3/pkg/stringhelpers"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-promote/pkg/environments"
	"k8s.io/client-go/kubernetes"

	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/naming"

	"github.com/pkg/errors"

	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"

	"github.com/blang/semver"
	typev1 "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned/typed/jenkins.io/v1"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	helm "github.com/jenkins-x/jx-helpers/v3/pkg/helmer"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	optionEnvironment         = "env"
	optionApplication         = "app"
	optionTimeout             = "timeout"
	optionPullRequestPollTime = "pull-request-poll-time"

	// DefaultChartRepo default URL for charts repository
	DefaultChartRepo = "http://jenkins-x-chartmuseum:8080"
)

var (
	waitAfterPullRequestCreated = time.Second * 3
)

// Options containers the CLI options
type Options struct {
	environments.EnvironmentPullRequestOptions

	Dir                     string
	Args                    []string
	Namespace               string
	DefaultAppNamespace     string
	Environment             string
	PromoteEnvironments     []string
	Application             string
	AppGitURL               string
	Pipeline                string
	Build                   string
	Version                 string
	VersionFile             string
	ReleaseName             string
	LocalHelmRepoName       string
	HelmRepositoryURL       string
	AutoMerge               bool
	NoHelmUpdate            bool
	All                     bool
	AllAutomatic            bool
	NoMergePullRequest      bool
	NoPoll                  bool
	NoWaitAfterMerge        bool
	IgnoreLocalFiles        bool
	NoWaitForUpdatePipeline bool
	DisableGitConfig        bool //  to disable git init in unit tests
	Timeout                 string
	PullRequestPollTime     string
	Filter                  string
	Alias                   string

	KubeClient kubernetes.Interface
	JXClient   versioned.Interface
	Helmer     helm.Helmer
	Input      input.Interface
	GitClient  gitclient.Interface

	// calculated fields
	TimeoutDuration         *time.Duration
	PullRequestPollDuration *time.Duration
	Activities              typev1.PipelineActivityInterface
	GitInfo                 *giturl.GitRepository
	releaseResource         *v1.Release
	ReleaseInfo             *ReleaseInfo
	prow                    bool

	// Used for testing
	CloneDir string
}

type ReleaseInfo struct {
	ReleaseName     string
	FullAppName     string
	Version         string
	PullRequestInfo *scm.PullRequest
}

var (
	promoteLong = templates.LongDesc(`
		Promotes a version of an application to zero to many permanent environments.

		For more documentation see: [https://jenkins-x.io/docs/getting-started/promotion/](https://jenkins-x.io/docs/getting-started/promotion/)

`)

	promoteExample = templates.Examples(`
		# Promote a version of the current application to staging
        # discovering the application name from the source code
		jx promote --version 1.2.3 --env staging

		# Promote a version of the myapp application to production
		jx promote --app myapp --version 1.2.3 --env production

		# To search for all the available charts for a given name use -f.
		# e.g. to find a redis chart to install
		jx promote -f redis

		# To promote a postgres chart using an alias
		jx promote -f postgres --alias mydb

		# To create or update a Preview Environment please see the 'jx preview' command if you are inside a git clone of a repo
		jx preview
	`)
)

// NewCmdPromote creates the new command for: jx get prompt
func NewCmdPromote() (*cobra.Command, *Options) {
	options := &Options{}
	cmd := &cobra.Command{
		Use:     "promote [application]",
		Short:   "Promotes a version of an application to an Environment",
		Long:    promoteLong,
		Example: promoteExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The Namespace to promote to")
	cmd.Flags().StringVarP(&options.Environment, optionEnvironment, "e", "", "The Environment to promote to")
	cmd.Flags().StringVarP(&options.DefaultAppNamespace, "default-app-namespace", "", "", "The default namespace for promoting to remote clusters for the first")
	cmd.Flags().StringArrayP("promotion-environments", "", options.PromoteEnvironments, "The environments considered for promotion")
	cmd.Flags().BoolVarP(&options.AllAutomatic, "all-auto", "", false, "Promote to all automatic environments in order")
	cmd.Flags().BoolVarP(&options.All, "all", "", false, "Promote to all automatic and manual environments in order using a draft PR for manual promotion environments")

	cmd.Flags().BoolVarP(&options.BatchMode, "batch-mode", "b", false, "Enables batch mode which avoids prompting for user input")

	options.AddOptions(cmd)
	return cmd, options
}

// AddOptions adds command level options to `promote`
func (o *Options) AddOptions(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Application, optionApplication, "a", "", "The Application to promote")
	cmd.Flags().StringVarP(&o.AppGitURL, "app-git-url", "", "", "The Git URL of the application being promoted. Only required if using file or kpt rules")
	cmd.Flags().StringVarP(&o.Filter, "filter", "f", "", "The search filter to find charts to promote")
	cmd.Flags().StringVarP(&o.Alias, "alias", "", "", "The optional alias used in the 'requirements.yaml' file")
	cmd.Flags().StringVarP(&o.Pipeline, "pipeline", "", "", "The Pipeline string in the form 'folderName/repoName/branch' which is used to update the PipelineActivity. If not specified its defaulted from  the '$BUILD_NUMBER' environment variable")
	cmd.Flags().StringVarP(&o.Build, "build", "", "", "The Build number which is used to update the PipelineActivity. If not specified its defaulted from  the '$BUILD_NUMBER' environment variable")
	cmd.Flags().StringVarP(&o.Version, "version", "v", "", "The Version to promote. If no version is specified it defaults to $VERSION which is usually populated in a pipeline. If no value can be found you will be prompted to pick the version")
	cmd.Flags().StringVarP(&o.VersionFile, "version-file", "", "", "the file to load the version from if not specified directly or via a $VERSION environment variable. Defaults to VERSION in the current dir")
	cmd.Flags().StringVarP(&o.LocalHelmRepoName, "helm-repo-name", "r", kube.LocalHelmRepoName, "The name of the helm repository that contains the app")
	cmd.Flags().StringVarP(&o.HelmRepositoryURL, "helm-repo-url", "u", "", "The Helm Repository URL to use for the App")
	cmd.Flags().StringVarP(&o.ReleaseName, "release", "", "", "The name of the helm release")
	cmd.Flags().StringVarP(&o.Timeout, optionTimeout, "t", "1h", "The timeout to wait for the promotion to succeed in the underlying Environment. The command fails if the timeout is exceeded or the promotion does not complete")
	cmd.Flags().StringVarP(&o.PullRequestPollTime, optionPullRequestPollTime, "", "20s", "Poll time when waiting for a Pull Request to merge")

	cmd.Flags().StringVarP(&o.DevEnvContext.GitUsername, "git-user", "", "", "Git username used to clone the development environment. If not specified its loaded from the git credentials file")
	cmd.Flags().StringVarP(&o.DevEnvContext.GitToken, "git-token", "", "", "Git token used to clone the development environment. If not specified its loaded from the git credentials file")

	cmd.Flags().BoolVarP(&o.NoHelmUpdate, "no-helm-update", "", false, "Allows the 'helm repo update' command if you are sure your local helm cache is up to date with the version you wish to promote")
	cmd.Flags().BoolVarP(&o.NoMergePullRequest, "no-merge", "", false, "Disables automatic merge of promote Pull Requests")

	cmd.Flags().BoolVarP(&o.NoPoll, "no-poll", "", false, "Disables polling for Pull Request or Pipeline status")
	cmd.Flags().BoolVarP(&o.NoWaitAfterMerge, "no-wait", "", false, "Disables waiting for completing promotion after the Pull request is merged")
	cmd.Flags().BoolVarP(&o.IgnoreLocalFiles, "ignore-local-file", "", false, "Ignores the local file system when deducing the Git repository")
	cmd.Flags().BoolVarP(&o.AutoMerge, "auto-merge", "", false, "If enabled add the 'updatebot' label to tell lighthouse to eagerly merge. Usually the Pull Request pipeline will add this label during the Pull Request pipeline after any extra generation/commits have been done and the PR is valid")
}

func (o *Options) hasApplicationFlag() bool {
	return o.Application != ""
}

func (o *Options) hasArgs() bool {
	return len(o.Args) > 0
}

func (o *Options) setApplicationNameFromArgs() {
	o.Application = o.Args[0]
}

func (o *Options) hasFilterFlag() bool {
	return o.Filter != ""
}

type searchForChartFn func(filter string) (string, error)

func (o *Options) setApplicationNameFromFilter(searchForChart searchForChartFn) error {
	app, err := searchForChart(o.Filter)
	if err != nil {
		return errors.Wrap(err, "searching app name in chart failed")
	}

	o.Application = app

	return nil
}

type discoverAppNameFn func() (string, error)

func (o *Options) setApplicationNameFromDiscoveredAppName(discoverAppName discoverAppNameFn) error {
	app, err := discoverAppName()
	if err != nil {
		return errors.Wrap(err, "discovering app name failed")
	}

	if !o.BatchMode {
		var continueWithAppName bool

		question := fmt.Sprintf("Are you sure you want to promote the application named: %s?", app)

		continueWithAppName, err := o.Input.Confirm(question, true, "please confirm you wish to promote this app")
		if err != nil {
			return errors.Wrapf(err, "failed to confirm promotion")
		}

		if !continueWithAppName {
			return errors.New("user canceled execution")
		}
	}

	o.Application = app

	return nil
}

// EnsureApplicationNameIsDefined validates if an application name flag was provided by the user. If missing it will
// try to set it up or return an error
func (o *Options) EnsureApplicationNameIsDefined(sf searchForChartFn, df discoverAppNameFn) error {
	if !o.hasApplicationFlag() && o.hasArgs() {
		o.setApplicationNameFromArgs()
	}
	if !o.hasApplicationFlag() && o.hasFilterFlag() {
		err := o.setApplicationNameFromFilter(sf)
		if err != nil {
			return err
		}
	}

	if !o.hasApplicationFlag() {
		return o.setApplicationNameFromDiscoveredAppName(df)
	}
	return nil
}

// Validate validates settings
func (o *Options) Validate() error {
	if o.Input == nil {
		o.Input = survey.NewInput()
	}
	var err error
	o.KubeClient, o.Namespace, err = kube.LazyCreateKubeClientAndNamespace(o.KubeClient, o.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to create the kube client")
	}
	o.JXClient, err = jxclient.LazyCreateJXClient(o.JXClient)
	if err != nil {
		return errors.Wrapf(err, "failed to create the jx client")
	}
	if o.VersionFile == "" {
		o.VersionFile = filepath.Join(o.Dir, "VERSION")
	}
	return nil
}

// Run implements this command
func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate options")
	}

	// TODO move to validate
	err = o.EnsureApplicationNameIsDefined(o.SearchForChart, o.DiscoverAppName)
	if err != nil {
		return err
	}

	if o.Version == "" {
		exists, err := files.FileExists(o.VersionFile)
		if err != nil {
			return errors.Wrapf(err, "failed to check for file %s", o.VersionFile)
		}
		if exists {
			data, err := ioutil.ReadFile(o.VersionFile)
			if err != nil {
				return errors.Wrapf(err, "failed to read version file %s", o.VersionFile)
			}
			o.Version = strings.TrimSpace(string(data))
		}
		if o.Version != "" {
			log.Logger().Infof("defaulting to the version %s from file %s", termcolor.ColorInfo(o.Version), termcolor.ColorInfo(o.VersionFile))
		}
		if o.Version == "" {
			o.Version = os.Getenv("VERSION")
			if o.Version != "" {
				log.Logger().Infof("defaulting to the version %s from $VERSION", termcolor.ColorInfo(o.Version))
			}
		}
	}
	if o.Version == "" && o.Application != "" {
		o.Version, err = o.findLatestVersion(o.Application)
		if err != nil {
			return errors.Wrapf(err, "failed to find latest version of app %s", o.Application)
		}
	}

	ns := o.Namespace
	if ns == "" {
		return errors.Errorf("no namespace defined")
	}
	jxClient := o.JXClient
	if o.GitClient == nil {
		o.GitClient = cli.NewCLIClient("", o.CommandRunner)
	}

	err = o.DevEnvContext.LazyLoad(o.GitClient, o.JXClient, o.Namespace, o.Git())
	if err != nil {
		return errors.Wrap(err, "failed to lazy load the EnvironmentContext")
	}

	if kube.IsInCluster() && !o.DisableGitConfig {
		err = o.InitGitConfigAndUser()
		if err != nil {
			return errors.Wrapf(err, "failed to init git")
		}
	}

	// TODO we should remove these flags
	o.prow = true
	o.NoWaitForUpdatePipeline = true

	if o.HelmRepositoryURL == "" {
		o.HelmRepositoryURL, err = o.ResolveChartRepositoryURL()
		if err != nil {
			return errors.Wrapf(err, "failed to resolve helm repository URL")
		}
	}
	if o.Environment == "" && !o.BatchMode {
		names := []string{}
		m, allEnvNames, err := jxenv.GetOrderedEnvironments(jxClient, ns)
		if err != nil {
			return err
		}
		for _, n := range allEnvNames {
			env := m[n]
			if env.Spec.Kind == v1.EnvironmentKindTypePermanent {
				names = append(names, n)
			}
		}
		o.Environment, err = o.Input.PickNameWithDefault(names, "Pick environment:", "", "please select an Environment name")
		if err != nil {
			return errors.Wrapf(err, "failed to pick an Environment name")
		}
	}

	if o.PullRequestPollTime != "" {
		duration, err := time.ParseDuration(o.PullRequestPollTime)
		if err != nil {
			return fmt.Errorf("Invalid duration format %s for option --%s: %s", o.PullRequestPollTime, optionPullRequestPollTime, err)
		}
		o.PullRequestPollDuration = &duration
	}
	if o.Timeout != "" {
		duration, err := time.ParseDuration(o.Timeout)
		if err != nil {
			return fmt.Errorf("Invalid duration format %s for option --%s: %s", o.Timeout, optionTimeout, err)
		}
		o.TimeoutDuration = &duration
	}

	targetNS, env, err := o.GetTargetNamespace(o.Namespace, o.Environment)
	if err != nil {
		return err
	}

	o.Activities = jxClient.JenkinsV1().PipelineActivities(ns)

	releaseName := o.ReleaseName
	if releaseName == "" {
		releaseName = targetNS + "-" + o.Application
		o.ReleaseName = releaseName
	}

	if len(o.PromoteEnvironments) > 0 {
		return o.PromoteAll(func(env *v1.Environment) bool {
			return Contains(o.PromoteEnvironments, env.Spec.Label)
		})
	}

	if o.All {
		return o.PromoteAll(func(env *v1.Environment) bool {
			return (env.Spec.PromotionStrategy == v1.PromotionStrategyTypeAutomatic || env.Spec.PromotionStrategy == v1.PromotionStrategyTypeManual) && env.Spec.Kind.IsPermanent()
		})
	}
	if o.AllAutomatic {
		return o.PromoteAll(func(env *v1.Environment) bool {
			return env.Spec.PromotionStrategy == v1.PromotionStrategyTypeAutomatic && env.Spec.Kind.IsPermanent()
		})
	}
	if env == nil {
		if o.Environment == "" {
			return options.MissingOption(optionEnvironment)
		}
		env, err := jxClient.JenkinsV1().Environments(ns).Get(context.TODO(), o.Environment, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if env == nil {
			return fmt.Errorf("Could not find an Environment called %s", o.Environment)
		}
	}
	releaseInfo, err := o.Promote(targetNS, env, true, false, o.NoPoll)
	if err != nil {
		return err
	}

	o.ReleaseInfo = releaseInfo
	if !o.NoPoll {
		err = o.WaitForPromotion(targetNS, env, releaseInfo)
		if err != nil {
			return err
		}
	}
	return err
}

func Contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

// DiscoverAppNam discovers an app name from a helm chart installation
func (o *Options) DiscoverAppName() (string, error) {
	answer := ""
	chartFile, err := o.FindHelmChartInDir("")
	if err != nil {
		return answer, err
	}
	if chartFile != "" {
		return helm.LoadChartName(chartFile)
	}

	gitInfo, err := gitdiscovery.FindGitInfoFromDir(o.Dir)
	if err != nil {
		return answer, err
	}
	if gitInfo == nil {
		return answer, fmt.Errorf("no git info found to discover app name from")
	}
	answer = gitInfo.Name
	return answer, nil
}

// FindHelmChartInDir finds the helm chart in the given dir. If no dir is specified then the current dir is used
func (o *Options) FindHelmChartInDir(dir string) (string, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", errors.Wrap(err, "failed to get the current working directory")
		}
	}
	h := o.Helm()
	h.SetCWD(dir)
	return h.FindChart()
}

// DefaultChartRepositoryURL returns the default chart repository URL
func (o *Options) DefaultChartRepositoryURL() string {
	chartRepo := os.Getenv("CHART_REPOSITORY")
	if chartRepo == "" {
		requirements := o.DevEnvContext.Requirements
		if requirements != nil {
			chartRepo = requirements.Cluster.ChartRepository
		}
	}
	if chartRepo == "" {
		if kube.IsInCluster() {
			log.Logger().Warnf("No $CHART_REPOSITORY defined so using the default value of: %s", DefaultChartRepo)
		}
		chartRepo = DefaultChartRepo
	}
	return chartRepo
}

func (o *Options) PromoteAll(pred func(*v1.Environment) bool) error {
	kubeClient := o.KubeClient
	currentNs := o.Namespace
	team, _, err := jxenv.GetDevNamespace(kubeClient, currentNs)
	if err != nil {
		return err
	}
	jxClient := o.JXClient
	envs, err := jxClient.JenkinsV1().Environments(team).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Logger().Warnf("No Environments found: %s/n", err)
		return nil
	}
	environments := envs.Items
	if len(environments) == 0 {
		log.Logger().Warnf("No Environments have been created yet in team %s. Please create some via 'jx create env'", team)
		return nil
	}
	jxenv.SortEnvironments(environments)

	for _, env := range environments {
		if pred(&env) {
			ns := env.Spec.Namespace
			if ns == "" {
				return fmt.Errorf("No namespace for environment %s", env.Name)
			}
			// lets clear the branch name so that we create a new branch for each PR...
			o.BranchName = ""
			releaseInfo, err := o.Promote(ns, &env, false, env.Spec.PromotionStrategy != v1.PromotionStrategyTypeAutomatic, o.NoPoll)
			if err != nil {
				return err
			}
			o.ReleaseInfo = releaseInfo
			if !o.NoPoll {
				err = o.WaitForPromotion(ns, &env, releaseInfo)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (o *Options) Promote(targetNS string, env *v1.Environment, warnIfAuto, draftPR, noPoll bool) (*ReleaseInfo, error) {
	app := o.Application
	if app == "" {
		log.Logger().Warnf("No application name could be detected so cannot promote via Helm. If the detection of the helm chart name is not working consider adding it with the --%s argument on the 'jx promote' command", optionApplication)
		return nil, nil
	}
	version := o.Version
	info := termcolor.ColorInfo
	if version == "" {
		log.Logger().Infof("Promoting latest version of app %s to namespace %s", info(app), info(targetNS))
	} else {
		log.Logger().Infof("Promoting app %s version %s to namespace %s", info(app), info(version), info(targetNS))
	}
	fullAppName := app
	if o.LocalHelmRepoName != "" {
		fullAppName = o.LocalHelmRepoName + "/" + app
	}
	releaseName := o.ReleaseName
	if releaseName == "" {
		releaseName = targetNS + "-" + app
		o.ReleaseName = releaseName
	}
	releaseInfo := &ReleaseInfo{
		ReleaseName: releaseName,
		FullAppName: fullAppName,
		Version:     version,
	}

	if warnIfAuto && env != nil && env.Spec.PromotionStrategy == v1.PromotionStrategyTypeAutomatic && !o.BatchMode {
		log.Logger().Infof("%s", termcolor.ColorWarning(fmt.Sprintf("WARNING: The Environment %s is setup to promote automatically as part of the CI/CD Pipelines.\n", env.Name)))
		flag, err := o.Input.Confirm("Do you wish to promote anyway? :", false, "usually we do not manually promote to Auto promotion environments")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to confirm promotion")
		}
		if !flag {
			return releaseInfo, nil
		}
	}

	jxClient := o.JXClient
	kubeClient := o.KubeClient
	promoteKey := o.CreatePromoteKey(env)
	if env != nil {
		if !env.Spec.Kind.IsPermanent() {
			return nil, errors.Errorf("cannot promote to Environment which is not a permanent Environment")
		}
		source := &env.Spec.Source
		if source.URL == "" && !env.Spec.RemoteCluster && o.DevEnvContext.DevEnv != nil {
			// lets default to the git repository of the dev environment as we are sharing the git repository across multiple namespaces
			source.URL = o.DevEnvContext.DevEnv.Spec.Source.URL
		}

		if source.URL != "" {
			err := o.PromoteViaPullRequest(env, releaseInfo, draftPR)
			if err == nil {
				startPromotePR := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromotePullRequestStep) error {
					activities.StartPromotionPullRequest(a, s, ps, p)
					pr := releaseInfo.PullRequestInfo
					if pr != nil && pr.Link != "" {
						p.PullRequestURL = pr.Link
					}
					if version != "" && a.Spec.Version == "" {
						a.Spec.Version = version
					}
					if noPoll {
						p.Status = v1.ActivityStatusTypeSucceeded
						ps.Status = v1.ActivityStatusTypeSucceeded
					}

					// if all steps are completed lets mark succeeded/failed
					activities.UpdateStatus(a, false, nil)
					return nil
				}
				err = promoteKey.OnPromotePullRequest(kubeClient, jxClient, o.Namespace, startPromotePR)
				if err != nil {
					log.Logger().Warnf("Failed to update PipelineActivity: %s", err)
				}
				// lets sleep a little before we try poll for the PR status
				time.Sleep(waitAfterPullRequestCreated)
			}
			return releaseInfo, err
		}
	}
	return nil, errors.Errorf("no source repository URL available on  environment %s", o.Environment)
}

// ResolveChartRepositoryURL resolves the current Chart Museum URL so we can pass it into a remote Environment's
// git repository
func (o *Options) ResolveChartRepositoryURL() (string, error) {
	kubeClient := o.KubeClient
	jxClient := o.JXClient
	ns := o.Namespace
	answer, err := services.FindServiceURL(kubeClient, ns, kube.ServiceChartMuseum)
	if err != nil && apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil || answer == "" {
		// lets try find a `chartmusem` ingress
		var err2 error
		answer, err2 = services.FindIngressURL(kubeClient, ns, "chartmuseum")
		if err2 != nil && apierrors.IsNotFound(err2) {
			err2 = nil
		}
		if err2 == nil && answer != "" {
			return answer, nil
		}
	}
	if answer == "" {
		env, err := jxenv.GetDevEnvironment(jxClient, ns)
		if err != nil && apierrors.IsNotFound(err) {
			err = nil
		}
		if env != nil {
			if o.GitClient == nil {
				o.GitClient = cli.NewCLIClient("", o.CommandRunner)
			}
			requirements, err := requirements.GetClusterRequirementsConfig(o.GitClient, jxClient)
			if err != nil {
				return answer, errors.Wrapf(err, "getting requirements from dev Environment")
			}
			if requirements != nil {
				if requirements.Cluster.ChartRepository != "" {
					return requirements.Cluster.ChartRepository, nil
				}
			}
		}
	}
	return answer, err
}

func (o *Options) GetTargetNamespace(ns string, env string) (string, *v1.Environment, error) {
	kubeClient := o.KubeClient
	currentNs := o.Namespace
	team, _, err := jxenv.GetDevNamespace(kubeClient, currentNs)
	if err != nil {
		return "", nil, err
	}

	jxClient := o.JXClient
	if err != nil {
		return "", nil, err
	}

	m, envNames, err := jxenv.GetEnvironments(jxClient, team)
	if err != nil {
		return "", nil, err
	}
	if len(envNames) == 0 {
		return "", nil, fmt.Errorf("No Environments have been created yet in team %s. Please create some via 'jx create env'", team)
	}

	var envResource *v1.Environment
	targetNS := currentNs
	if env != "" {
		envResource = m[env]
		if envResource == nil {
			return "", nil, options.InvalidOption(optionEnvironment, env, envNames)
		}
		targetNS = envResource.Spec.Namespace
		if targetNS == "" {
			return "", nil, fmt.Errorf("environment %s does not have a namespace associated with it!", env)
		}
	} else if ns != "" {
		targetNS = ns
	}

	labels := map[string]string{}
	annotations := map[string]string{}
	err = jxenv.EnsureNamespaceCreated(kubeClient, targetNS, labels, annotations)
	if err != nil {
		return "", nil, err
	}
	return targetNS, envResource, nil
}

func (o *Options) WaitForPromotion(ns string, env *v1.Environment, releaseInfo *ReleaseInfo) error {
	if o.TimeoutDuration == nil {
		log.Logger().Infof("No --%s option specified on the 'jx promote' command so not waiting for the promotion to succeed", optionTimeout)
		return nil
	}
	if o.PullRequestPollDuration == nil {
		log.Logger().Infof("No --%s option specified on the 'jx promote' command so not waiting for the promotion to succeed", optionPullRequestPollTime)
		return nil
	}
	duration := *o.TimeoutDuration
	end := time.Now().Add(duration)

	jxClient := o.JXClient
	kubeClient := o.KubeClient
	pullRequestInfo := releaseInfo.PullRequestInfo
	if pullRequestInfo != nil {
		promoteKey := o.CreatePromoteKey(env)

		err := o.waitForGitOpsPullRequest(ns, env, releaseInfo, end, duration, promoteKey)
		if err != nil {
			// TODO based on if the PR completed or not fail the PR or the Promote?
			promoteKey.OnPromotePullRequest(kubeClient, jxClient, o.Namespace, activities.FailedPromotionPullRequest)
			return err
		}
	}
	return nil
}

// TODO This could do with a refactor and some tests...
func (o *Options) waitForGitOpsPullRequest(ns string, env *v1.Environment, releaseInfo *ReleaseInfo, end time.Time, duration time.Duration, promoteKey *activities.PromoteStepActivityKey) error {
	pullRequestInfo := releaseInfo.PullRequestInfo
	logMergeFailure := false
	logNoMergeCommitSha := false
	logHasMergeSha := false
	logMergeStatusError := false
	logNoMergeStatuses := false
	urlStatusMap := map[string]scm.State{}
	urlStatusTargetURLMap := map[string]string{}

	jxClient := o.JXClient
	if jxClient == nil {
		return errors.Errorf("no jx client")
	}
	kubeClient := o.KubeClient
	if kubeClient == nil {
		return errors.Errorf("no kube client")
	}

	scmClient := o.ScmClient
	if scmClient == nil {
		return errors.Errorf("no ScmClient")
	}

	ctx := context.Background()

	if pullRequestInfo != nil {
		fullName := pullRequestInfo.Repository().FullName
		prNumber := pullRequestInfo.Number
		for {
			pr, _, err := scmClient.PullRequests.Find(ctx, fullName, prNumber)
			if err != nil {
				return errors.Wrapf(err, "failed to find PR %s %d", fullName, prNumber)
			}
			if err != nil {
				log.Logger().Warnf("failed to find PR %s %d: %s", fullName, prNumber, err.Error())
			} else {
				if pr.Merged {
					if pr.MergeSha == "" {
						if !logNoMergeCommitSha {
							logNoMergeCommitSha = true
							log.Logger().Infof("Pull Request %s is merged but waiting for Merge SHA", termcolor.ColorInfo(pr.Link))
						}
					} else {
						mergeSha := pr.MergeSha
						if !logHasMergeSha {
							logHasMergeSha = true
							log.Logger().Infof("Pull Request %s is merged at sha %s", termcolor.ColorInfo(pr.Link), termcolor.ColorInfo(mergeSha))

							mergedPR := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromotePullRequestStep) error {
								activities.CompletePromotionPullRequest(a, s, ps, p)
								p.MergeCommitSHA = mergeSha
								return nil
							}
							promoteKey.OnPromotePullRequest(kubeClient, jxClient, o.Namespace, mergedPR)

							if o.NoWaitAfterMerge {
								log.Logger().Infof("Pull requests are merged, No wait on promotion to complete")
								return err
							}
						}

						promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, activities.StartPromotionUpdate)

						if o.NoWaitForUpdatePipeline {
							err = o.CommentOnIssues(ns, env, promoteKey)
							if err == nil {
								err = promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, activities.CompletePromotionUpdate)
							}
							return err
						}

						statuses, _, err := scmClient.Repositories.ListStatus(ctx, fullName, mergeSha, scm.ListOptions{})
						if err != nil {
							if !logMergeStatusError {
								logMergeStatusError = true
								log.Logger().Warnf("Failed to query merge status of repo %s with merge sha %s due to: %s", fullName, mergeSha, err)
							}
						} else {
							if len(statuses) == 0 {
								if !logNoMergeStatuses {
									logNoMergeStatuses = true
									log.Logger().Infof("Merge commit has not yet any statuses on repo %s merge sha %s", fullName, mergeSha)
								}
							} else {
								for _, status := range statuses {
									if status.State == scm.StateFailure {
										log.Logger().Warnf("merge status: %s URL: %s description: %s",
											status.State, status.Target, status.Desc)
										return fmt.Errorf("Status: %s URL: %s description: %s\n",
											status.State, status.Target, status.Desc)
									}
									url := status.Link
									if url == "" {
										url = status.Target
									}
									state := status.State
									if urlStatusMap[url] == scm.StateUnknown || urlStatusMap[url] != scm.StateSuccess {
										if urlStatusMap[url] != state {
											urlStatusMap[url] = state
											urlStatusTargetURLMap[url] = status.Target
											log.Logger().Infof("merge status: %s for URL %s with target: %s description: %s",
												termcolor.ColorInfo(state), termcolor.ColorInfo(url), termcolor.ColorInfo(status.Target), termcolor.ColorInfo(status.Desc))
										}
									}
								}
								prStatuses := []v1.GitStatus{}
								keys := []string{}
								for k := range urlStatusMap {
									keys = append(keys, k)
								}
								sort.Strings(keys)
								for _, url := range keys {
									state := urlStatusMap[url]
									targetURL := urlStatusTargetURLMap[url]
									if targetURL == "" {
										targetURL = url
									}
									prStatuses = append(prStatuses, v1.GitStatus{
										URL:    targetURL,
										Status: state.String(),
									})
								}
								updateStatuses := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromoteUpdateStep) error {
									p.Statuses = prStatuses
									return nil
								}
								promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, updateStatuses)

								succeeded := true
								for _, v := range urlStatusMap {
									if v != scm.StateSuccess {
										succeeded = false
									}
								}
								if succeeded {
									log.Logger().Info("Merge status checks all passed so the promotion worked!")
									err = o.CommentOnIssues(ns, env, promoteKey)
									if err == nil {
										err = promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, activities.CompletePromotionUpdate)
									}
									return err
								}
							}
						}
					}
				} else {
					if pr.Closed {
						log.Logger().Warnf("Pull Request %s is closed", termcolor.ColorInfo(pr.Link))
						return fmt.Errorf("Promotion failed as Pull Request %s is closed without merging", pr.Link)
					}

					prLastCommitSha := o.pullRequestLastCommitSha(pr)

					status, err := o.PullRequestLastCommitStatus(pr)
					if err != nil || status == nil {
						log.Logger().Warnf("Failed to query the Pull Request last commit status for %s ref %s %s", pr.Link, prLastCommitSha, err)
						//return fmt.Errorf("Failed to query the Pull Request last commit status for %s ref %s %s", pr.Link, prLastCommitSha, err)
						//} else if status.State == "in-progress" {
					} else if StateIsPending(status) {
						log.Logger().Info("The build for the Pull Request last commit is currently in progress.")
					} else {
						if status.State == scm.StateSuccess {
							if !(o.NoMergePullRequest) {
								tideMerge := false
								// Now check if tide is running or not
								commitStatues, _, err := scmClient.Repositories.ListStatus(ctx, fullName, prLastCommitSha, scm.ListOptions{})
								if err != nil {
									log.Logger().Warnf("unable to get commit statuses for %s", pr.Link)
								} else {
									for _, s := range commitStatues {
										if s.Label == "tide" {
											tideMerge = true
											break
										}
									}
								}
								if !tideMerge {
									prMergeOptions := &scm.PullRequestMergeOptions{
										CommitTitle: "jx promote automatically merged promotion PR",
									}
									_, err = scmClient.PullRequests.Merge(ctx, fullName, prNumber, prMergeOptions)
									// TODO
									//err = gitProvider.MergePullRequest(pr, "jx promote automatically merged promotion PR")
									if err != nil {
										if !logMergeFailure {
											logMergeFailure = true
											log.Logger().Warnf("Failed to merge the Pull Request %s due to %s maybe I don't have karma?", pr.Link, err)
										}
									}
								}
							}
						} else if StateIsErrorOrFailure(status) {
							return fmt.Errorf("Pull request %s last commit has status %s for ref %s", pr.Link, status.State.String(), prLastCommitSha)
						} else {
							log.Logger().Infof("got git provider status %s from PR %s", status.State.String(), pr.Link)
						}
					}
				}
				if !pr.Mergeable {
					log.Logger().Info("Rebasing PullRequest due to conflict")

					err = o.PromoteViaPullRequest(env, releaseInfo, false)
					if releaseInfo.PullRequestInfo != nil {
						pullRequestInfo = releaseInfo.PullRequestInfo
					}
				}
			}
			if time.Now().After(end) {
				return fmt.Errorf("Timed out waiting for pull request %s to merge. Waited %s", pr.Link, duration.String())
			}
			time.Sleep(*o.PullRequestPollDuration)
		}
	}
	return nil
}

func StateIsErrorOrFailure(status *scm.Status) bool {
	switch status.State {
	case scm.StateCanceled, scm.StateError, scm.StateFailure:
		return true
	default:
		return false
	}
}

func StateIsPending(status *scm.Status) bool {
	switch status.State {
	case scm.StatePending, scm.StateRunning:
		return true
	default:
		return false
	}
}

func (o *Options) PullRequestLastCommitStatus(pr *scm.PullRequest) (*scm.Status, error) {
	scmClient := o.ScmClient
	if scmClient == nil {
		return nil, errors.Errorf("no ScmClient")
	}

	ctx := context.Background()

	fullName := pr.Repository().FullName

	prLastCommitSha := o.pullRequestLastCommitSha(pr)

	// lets try merge if the status is good
	statuses, _, err := scmClient.Repositories.ListStatus(ctx, fullName, prLastCommitSha, scm.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query repository %s for PR last commit status of %s", fullName, prLastCommitSha)
	}
	if len(statuses) == 0 {
		return nil, errors.Errorf("no commit statuses returned for repository %s for PR last commit status of %s", fullName, prLastCommitSha)
	}
	// TODO how to find the last status - assume the first?
	return statuses[0], nil
}

func (o *Options) pullRequestLastCommitSha(pr *scm.PullRequest) string {
	return pr.Head.Sha
}

func (o *Options) findLatestVersion(app string) (string, error) {
	charts, err := o.Helm().SearchCharts(app, true)
	if err != nil {
		return "", err
	}

	var maxSemVer *semver.Version
	maxString := ""
	for _, chart := range charts {
		sv, err := semver.Parse(chart.ChartVersion)
		if err != nil {
			log.Logger().Warnf("Invalid semantic version: %s %s", chart.ChartVersion, err)
			if maxString == "" || strings.Compare(chart.ChartVersion, maxString) > 0 {
				maxString = chart.ChartVersion
			}
		} else {
			if maxSemVer == nil || maxSemVer.Compare(sv) > 0 {
				maxSemVer = &sv
			}
		}
	}

	if maxSemVer != nil {
		return maxSemVer.String(), nil
	}
	if maxString == "" {
		return "", fmt.Errorf("Could not find a version of app %s in the helm repositories", app)
	}
	return maxString, nil
}

// Helm lazily create a helmer
func (o *Options) Helm() helm.Helmer {
	if o.Helmer == nil {
		o.Helmer = helm.NewHelmCLI("")
	}
	return o.Helmer
}

func (o *Options) CreatePromoteKey(env *v1.Environment) *activities.PromoteStepActivityKey {
	pipeline := o.Pipeline
	if o.Build == "" {
		o.Build = builds.GetBuildNumber()
	}
	build := o.Build
	buildURL := os.Getenv("BUILD_URL")
	buildLogsURL := os.Getenv("BUILD_LOG_URL")
	releaseNotesURL := ""
	gitInfo := o.GitInfo
	if !o.IgnoreLocalFiles {
		var err error
		if gitInfo == nil {
			o.GitInfo, err = gitdiscovery.FindGitInfoFromDir(o.Dir)
			if err != nil {
				log.Logger().Warnf("Could not discover the Git repository info %s", err)
			}
		}

		releaseName := o.ReleaseName
		if o.releaseResource == nil && releaseName != "" {
			jxClient := o.JXClient
			if err == nil && jxClient != nil {
				release, err := jxClient.JenkinsV1().Releases(env.Spec.Namespace).Get(context.TODO(), releaseName, metav1.GetOptions{})
				if err == nil && release != nil {
					o.releaseResource = release
				}
			}
		}
		if o.releaseResource != nil {
			releaseNotesURL = o.releaseResource.Spec.ReleaseNotesURL
		}
	}
	if pipeline == "" {
		pipeline, build = o.GetPipelineName(gitInfo, pipeline, build, o.Application)
	}
	if pipeline != "" && build == "" {
		log.Logger().Warnf("No $BUILD_NUMBER environment variable found so cannot record promotion activities into the PipelineActivity resources in kubernetes")
		var err error
		build, err = o.GetLatestPipelineBuildByCRD(pipeline)
		if err != nil {
			log.Logger().Warnf("Could not discover the latest PipelineActivity build %s", err)
		}
	}
	name := pipeline
	if build != "" {
		name += "-" + build
	}
	name = naming.ToValidName(name)
	log.Logger().Debugf("Using pipeline: %s build: %s", termcolor.ColorInfo(pipeline), termcolor.ColorInfo("#"+build))
	return &activities.PromoteStepActivityKey{
		PipelineActivityKey: activities.PipelineActivityKey{
			Name:            name,
			Pipeline:        pipeline,
			Build:           build,
			BuildURL:        buildURL,
			BuildLogsURL:    buildLogsURL,
			GitInfo:         gitInfo,
			ReleaseNotesURL: releaseNotesURL,
		},
		Environment: env.Name,
	}
}

// GetLatestPipelineBuildByCRD returns the latest pipeline build
func (o *Options) GetLatestPipelineBuildByCRD(pipeline string) (string, error) {
	// lets find the latest build number
	jxClient := o.JXClient
	ns := o.Namespace
	pipelines, err := jxClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	buildNumber := 0
	for _, p := range pipelines.Items {
		if p.Spec.Pipeline == pipeline {
			b := p.Spec.Build
			if b != "" {
				n, err := strconv.Atoi(b)
				if err == nil {
					if n > buildNumber {
						buildNumber = n
					}
				}
			}
		}
	}
	if buildNumber > 0 {
		return strconv.Itoa(buildNumber), nil
	}
	return "1", nil
}

// GetPipelineName return the pipeline name
func (o *Options) GetPipelineName(gitInfo *giturl.GitRepository, pipeline string, build string, appName string) (string, string) {
	if build == "" {
		build = builds.GetBuildNumber()
	}
	branch := os.Getenv("BRANCH_NAME")
	if branch == "" || branch == "HEAD" {
		var err error
		// lets default the pipeline name from the Git repo
		branch, err = gitclient.Branch(o.Git(), ".")
		if err != nil {
			log.Logger().Warnf("Could not find the branch name: %s", err)
		}
	}
	if branch == "" || branch == "HEAD" {
		branch = os.Getenv("PULL_BASE_REF")
	}
	if branch == "" {
		branch = "master"
	}
	if gitInfo != nil && pipeline == "" {
		pipeline = stringhelpers.UrlJoin(gitInfo.Organisation, gitInfo.Name, branch)
	}
	if pipeline == "" && appName != "" {
		suffix := appName + "/" + branch

		// lets try deduce the pipeline name via the app name
		jxClient := o.JXClient
		ns := o.Namespace
		pipelineList, err := jxClient.JenkinsV1().PipelineActivities(ns).List(context.TODO(), metav1.ListOptions{})
		if err == nil {
			for _, pipelineResource := range pipelineList.Items {
				pipelineName := pipelineResource.Spec.Pipeline
				if strings.HasSuffix(pipelineName, suffix) {
					pipeline = pipelineName
					break
				}
			}
		}
	}
	if pipeline == "" {
		// lets try find
		log.Logger().Warnf("No $JOB_NAME environment variable found so cannot record promotion activities into the PipelineActivity resources in kubernetes")
	} else if build == "" {
		// lets validate and determine the current active pipeline branch
		p, b, err := o.GetLatestPipelineBuild(pipeline)
		if err != nil {
			log.Logger().Warnf("Failed to try detect the current Jenkins pipeline for %s due to %s", pipeline, err)
			build = "1"
		} else {
			pipeline = p
			build = b
		}
	}
	return pipeline, build
}

// getLatestPipelineBuild for the given pipeline name lets try find the Jenkins Pipeline and the latest build
func (o *Options) GetLatestPipelineBuild(pipeline string) (string, string, error) {
	log.Logger().Infof("pipeline %s", pipeline)
	build := ""
	jxClient := o.JXClient
	ns := o.Namespace
	kubeClient := o.KubeClient
	devEnv, err := jxenv.GetEnrichedDevEnvironment(kubeClient, jxClient, ns)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to find dev env")
	}
	webhookEngine := devEnv.Spec.WebHookEngine
	if webhookEngine == v1.WebHookEngineLighthouse {
		return pipeline, build, nil
	}
	return pipeline, build, nil
}

// CommentOnIssues comments on any issues for a release that the fix is available in the given environment
func (o *Options) CommentOnIssues(targetNS string, environment *v1.Environment, promoteKey *activities.PromoteStepActivityKey) error {
	ens := environment.Spec.Namespace
	envName := environment.Spec.Label
	app := o.Application
	version := o.Version
	if ens == "" {
		log.Logger().Warnf("Environment %s has no namespace", envName)
		return nil
	}
	if app == "" {
		log.Logger().Warnf("No application name so cannot comment on issues that they are now in %s", envName)
		return nil
	}
	if version == "" {
		log.Logger().Warnf("No version name so cannot comment on issues that they are now in %s", envName)
		return nil
	}
	gitInfo := o.GitInfo
	if gitInfo == nil {
		log.Logger().Warnf("No GitInfo discovered so cannot comment on issues that they are now in %s", envName)
		return nil
	}

	var err error
	releaseName := naming.ToValidNameWithDots(app + "-" + version)
	jxClient := o.JXClient
	kubeClient := o.KubeClient

	appNames := []string{app, o.ReleaseName, ens + "-" + app}
	url := ""
	for _, n := range appNames {
		url, err = services.FindServiceURL(kubeClient, ens, naming.ToValidName(n))
		if url != "" {
			break
		}
	}
	if url == "" {
		log.Logger().Warnf("Could not find the service URL in namespace %s for names %s", ens, strings.Join(appNames, ", "))
	}
	available := ""
	if url != "" {
		available = fmt.Sprintf(" and available [here](%s)", url)
	}

	if available == "" {
		ing, err := kubeClient.ExtensionsV1beta1().Ingresses(ens).Get(context.TODO(), app, metav1.GetOptions{})
		if err != nil || ing == nil && o.ReleaseName != "" && o.ReleaseName != app {
			ing, err = kubeClient.ExtensionsV1beta1().Ingresses(ens).Get(context.TODO(), o.ReleaseName, metav1.GetOptions{})
		}
		if ing != nil {
			if len(ing.Spec.Rules) > 0 {
				hostname := ing.Spec.Rules[0].Host
				if hostname != "" {
					available = fmt.Sprintf(" and available at %s", hostname)
					url = hostname
				}
			}
		}
	}

	// lets try update the PipelineActivity
	if url != "" && promoteKey.ApplicationURL == "" {
		promoteKey.ApplicationURL = url
		log.Logger().Debugf("Application is available at: %s", termcolor.ColorInfo(url))
	}

	release, err := jxClient.JenkinsV1().Releases(ens).Get(context.TODO(), releaseName, metav1.GetOptions{})
	if err == nil && release != nil {
		o.releaseResource = release
		issues := release.Spec.Issues

		versionMessage := version
		if release.Spec.ReleaseNotesURL != "" {
			versionMessage = "[" + version + "](" + release.Spec.ReleaseNotesURL + ")"
		}
		for _, issue := range issues {
			if issue.IsClosed() {
				log.Logger().Infof("Commenting that issue %s is now in %s", termcolor.ColorInfo(issue.URL), termcolor.ColorInfo(envName))

				comment := fmt.Sprintf(":white_check_mark: the fix for this issue is now deployed to **%s** in version %s %s", envName, versionMessage, available)
				id := issue.ID
				if id != "" {
					number, err := strconv.Atoi(id)
					if err != nil {
						log.Logger().Warnf("Could not parse issue id %s for URL %s", id, issue.URL)
					} else {
						if number > 0 {
							ctx := context.Background()
							fullName := scm.Join(gitInfo.Organisation, gitInfo.Name)
							input := &scm.CommentInput{
								Body: comment,
							}
							_, _, err = o.ScmClient.Issues.CreateComment(ctx, fullName, number, input)
							if err != nil {
								log.Logger().Warnf("Failed to add comment to issue %s: %s", issue.URL, err)
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func (o *Options) SearchForChart(filter string) (string, error) {
	answer := ""
	charts, err := o.Helm().SearchCharts(filter, false)
	if err != nil {
		return answer, err
	}
	if len(charts) == 0 {
		return answer, fmt.Errorf("No charts available for search filter: %s", filter)
	}
	m := map[string]*helm.ChartSummary{}
	names := []string{}
	for i, chart := range charts {
		text := chart.Name
		if chart.Description != "" {
			text = fmt.Sprintf("%-36s: %s", chart.Name, chart.Description)
		}
		names = append(names, text)
		m[text] = &charts[i]
	}
	name, err := o.Input.PickNameWithDefault(names, "Pick chart to promote: ", "", "which chart name do you wish to promote")
	if err != nil {
		return answer, err
	}
	chart := m[name]
	chartName := chart.Name
	// TODO now we split the chart into name and repo
	parts := strings.Split(chartName, "/")
	if len(parts) != 2 {
		return answer, fmt.Errorf("Invalid chart name '%s' was expecting single / character separating repo name and chart name", chartName)
	}
	repoName := parts[0]
	appName := parts[1]

	repos, err := o.Helm().ListRepos()
	if err != nil {
		return answer, err
	}

	repoUrl := repos[repoName]
	if repoUrl == "" {
		return answer, fmt.Errorf("Failed to find helm chart repo URL for '%s' when possible values are %s", repoName, stringhelpers.SortedMapKeys(repos))

	}
	o.Version = chart.ChartVersion
	o.HelmRepositoryURL = repoUrl
	return appName, nil
}

func (o *Options) InitGitConfigAndUser() error {
	_, so := setup.NewCmdGitSetup()

	so.KubeClient = o.KubeClient
	so.Namespace = o.Namespace
	so.CommandRunner = o.CommandRunner
	// TODO configure more values?
	err := so.Run()
	if err != nil {
		return errors.Wrapf(err, "failed to setup git config")
	}
	return nil

	/* TODO

	// lets make sure the home dir exists
	dir := util.HomeDir()
	err := os.MkdirAll(dir, files.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to make sure the home directory %s was created", dir)
	}

	// lets validate we have git configured
	_, _, err = gits.EnsureUserAndEmailSetup(o.Git())
	if err != nil {
		return err
	}

	err = githelpers.GitCommand(".", "git", "config", "--global", "credential.helper", "store")
	if err != nil {
		return errors.Wrapf(err, "failed to setup git")
	}
	if os.Getenv("XDG_CONFIG_HOME") == "" {
		log.Logger().Warnf("Note that the environment variable $XDG_CONFIG_HOME is not defined so we may not be able to push to git!")
	}
	return nil

	*/
}

func (o *Options) GetEnvChartValues(targetNS string, env *v1.Environment) ([]string, []string) {
	kind := string(env.Spec.Kind)
	values := []string{
		fmt.Sprintf("tags.jx-ns-%s=true", targetNS),
		fmt.Sprintf("global.jxNs%s=true", stringhelpers.ToCamelCase(targetNS)),
		fmt.Sprintf("tags.jx-%s=true", strings.ToLower(kind)),
		fmt.Sprintf("tags.jx-env-%s=true", env.ObjectMeta.Name),
		fmt.Sprintf("global.jx%s=true", kind),
		fmt.Sprintf("global.jxEnv%s=true", stringhelpers.ToCamelCase(env.ObjectMeta.Name)),
	}
	valueString := []string{
		fmt.Sprintf("global.jxNs=%s", targetNS),
		fmt.Sprintf("global.jxTypeEnv=%s", strings.ToLower(kind)),
		fmt.Sprintf("global.jxEnv=%s", env.ObjectMeta.Name),
	}
	if env.Spec.Kind == v1.EnvironmentKindTypePreview {
		valueString = append(valueString,
			fmt.Sprintf("global.jxPreviewApp=%s", env.Spec.PreviewGitSpec.ApplicationName),
			fmt.Sprintf("global.jxPreviewPr=%s", env.Spec.PreviewGitSpec.Name),
		)
	}
	return values, valueString
}
