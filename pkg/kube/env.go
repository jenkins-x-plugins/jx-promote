package kube

import (
	"fmt"
	"sort"

	v1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx/v2/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"gopkg.in/AlecAivazis/survey.v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	DefaultQuickstartLocations = []v1.QuickStartLocation{
		{
			GitURL:   gits.GitHubURL,
			GitKind:  gits.KindGitHub,
			Owner:    "jenkins-x-quickstarts",
			Includes: []string{"*"},
			Excludes: []string{"WIP-*"},
		},
	}
)

// GetDevEnvironment returns the current development environment using the jxClient for the given ns.
// If the Dev Environment cannot be found, returns nil Environment (rather than an error). A non-nil error is only
// returned if there is an error fetching the Dev Environment.
func GetDevEnvironment(jxClient versioned.Interface, ns string) (*v1.Environment, error) {
	//Find the settings for the team
	environmentInterface := jxClient.JenkinsV1().Environments(ns)
	name := LabelValueDevEnvironment
	answer, err := environmentInterface.Get(name, metav1.GetOptions{})
	if err == nil {
		return answer, nil
	}
	selector := "env=dev"
	envList, err := environmentInterface.List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	if len(envList.Items) == 1 {
		return &envList.Items[0], nil
	}
	if len(envList.Items) == 0 {
		return nil, nil
	}
	return nil, fmt.Errorf("Error fetching dev environment resource definition in namespace %s, No Environment called: %s or with selector: %s found %d entries: %v",
		ns, name, selector, len(envList.Items), envList.Items)
}

// GetDevNamespace returns the developer environment namespace
// which is the namespace that contains the Environments and the developer tools like Jenkins
func GetDevNamespace(kubeClient kubernetes.Interface, ns string) (string, string, error) {
	env := ""
	namespace, err := kubeClient.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil {
		return ns, env, err
	}
	if namespace == nil {
		return ns, env, fmt.Errorf("No namespace found for %s", ns)
	}
	if namespace.Labels != nil {
		answer := namespace.Labels[LabelTeam]
		if answer != "" {
			ns = answer
		}
		env = namespace.Labels[LabelEnvironment]
	}
	return ns, env, nil
}

// GetEnrichedDevEnvironment lazily creates the dev namespace if it does not already exist and
// auto-detects the webhook engine if its not specified
func GetEnrichedDevEnvironment(kubeClient kubernetes.Interface, jxClient versioned.Interface, ns string) (*v1.Environment, error) {
	env, err := EnsureDevEnvironmentSetup(jxClient, ns)
	if err != nil {
		return env, err
	}
	if env.Spec.WebHookEngine == v1.WebHookEngineNone {
		env.Spec.WebHookEngine = v1.WebHookEngineProw
	}
	return env, nil
}

// EnsureDevEnvironmentSetup ensures that the Environment is created in the given namespace
func EnsureDevEnvironmentSetup(jxClient versioned.Interface, ns string) (*v1.Environment, error) {
	// lets ensure there is a dev Environment setup so that we can easily switch between all the environments
	env, err := jxClient.JenkinsV1().Environments(ns).Get(LabelValueDevEnvironment, metav1.GetOptions{})
	if err != nil {
		// lets create a dev environment
		env = CreateDefaultDevEnvironment(ns)
		env, err = jxClient.JenkinsV1().Environments(ns).Create(env)
		if err != nil {
			return nil, err
		}
	}
	return env, nil
}

// CreateDefaultDevEnvironment creates a default development environment
func CreateDefaultDevEnvironment(ns string) *v1.Environment {
	return &v1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   LabelValueDevEnvironment,
			Labels: map[string]string{LabelTeam: ns, LabelEnvironment: LabelValueDevEnvironment},
		},
		Spec: v1.EnvironmentSpec{
			Namespace:         ns,
			Label:             "Development",
			PromotionStrategy: v1.PromotionStrategyTypeNever,
			Kind:              v1.EnvironmentKindTypeDevelopment,
			TeamSettings: v1.TeamSettings{
				UseGitOps:           true,
				AskOnCreate:         false,
				QuickstartLocations: DefaultQuickstartLocations,
				PromotionEngine:     v1.PromotionEngineJenkins,
				AppsRepository:      DefaultChartMuseumURL,
			},
		},
	}
}

// Ensure that the namespace exists for the given name
func EnsureNamespaceCreated(kubeClient kubernetes.Interface, name string, labels map[string]string, annotations map[string]string) error {
	n, err := kubeClient.CoreV1().Namespaces().Get(name, metav1.GetOptions{})
	if err == nil {
		// lets check if we have the labels setup
		if n.Annotations == nil {
			n.Annotations = map[string]string{}
		}
		if n.Labels == nil {
			n.Labels = map[string]string{}
		}
		changed := false
		if labels != nil {
			for k, v := range labels {
				if n.Labels[k] != v {
					n.Labels[k] = v
					changed = true
				}
			}
		}
		if annotations != nil {
			for k, v := range annotations {
				if n.Annotations[k] != v {
					n.Annotations[k] = v
					changed = true
				}
			}
		}
		if changed {
			_, err = kubeClient.CoreV1().Namespaces().Update(n)
			if err != nil {
				return fmt.Errorf("Failed to label Namespace %s %s", name, err)
			}
		}
		return nil
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
	}
	_, err = kubeClient.CoreV1().Namespaces().Create(namespace)
	if err != nil {
		return fmt.Errorf("Failed to create Namespace %s %s", name, err)
	} else {
		log.Logger().Infof("Namespace %s created ", name)
	}
	return err
}

// GetOrderedEnvironments returns a map of the environments along with the correctly ordered  names
func GetOrderedEnvironments(jxClient versioned.Interface, ns string) (map[string]*v1.Environment, []string, error) {
	m := map[string]*v1.Environment{}

	envNames := []string{}
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return m, envNames, err
	}
	SortEnvironments(envs.Items)
	for _, env := range envs.Items {
		n := env.Name
		copy := env
		m[n] = &copy
		if n != "" {
			envNames = append(envNames, n)
		}
	}
	return m, envNames, nil
}

// GetEnvironments returns a map of the environments along with a sorted list of names
func GetEnvironments(jxClient versioned.Interface, ns string) (map[string]*v1.Environment, []string, error) {
	m := map[string]*v1.Environment{}

	envNames := []string{}
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return m, envNames, err
	}
	for _, env := range envs.Items {
		n := env.Name
		copy := env
		m[n] = &copy
		if n != "" {
			envNames = append(envNames, n)
		}
	}
	sort.Strings(envNames)
	return m, envNames, nil
}

// GetEnvironment find an environment by name
func GetEnvironment(jxClient versioned.Interface, ns string, name string) (*v1.Environment, error) {
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, env := range envs.Items {
		if env.GetName() == name {
			return &env, nil
		}
	}
	return nil, fmt.Errorf("no environment with name '%s' found", name)
}

type ByOrder []v1.Environment

func (a ByOrder) Len() int      { return len(a) }
func (a ByOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByOrder) Less(i, j int) bool {
	env1 := a[i]
	env2 := a[j]
	o1 := env1.Spec.Order
	o2 := env2.Spec.Order
	if o1 == o2 {
		return env1.Name < env2.Name
	}
	return o1 < o2
}

func SortEnvironments(environments []v1.Environment) {
	sort.Sort(ByOrder(environments))
}

func PickEnvironment(envNames []string, defaultEnv string, handles util.IOFileHandles) (string, error) {
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	name := ""
	if len(envNames) == 0 {
		return "", nil
	} else if len(envNames) == 1 {
		name = envNames[0]
	} else {
		prompt := &survey.Select{
			Message: "Pick environment:",
			Options: envNames,
			Default: defaultEnv,
		}
		err := survey.AskOne(prompt, &name, nil, surveyOpts)
		if err != nil {
			return "", err
		}
	}
	return name, nil
}
