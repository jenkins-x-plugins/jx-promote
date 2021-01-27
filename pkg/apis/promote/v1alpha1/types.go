package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Promote represents the boot configuration
//
// +k8s:openapi-gen=true
type Promote struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the boot configuration
	// +optional
	Spec PromoteSpec `json:"spec"`
}

// PromoteSpec defines the desired state of Promote.
type PromoteSpec struct {

	// File specifies a promotion rule for a File such as for a Makefile or shell script
	FileRule *FileRule `json:"fileRule,omitempty"`

	// HelmRule specifies a composite helm chart to promote to by adding the app to the charts
	// 'requirements.yaml' file
	HelmRule *HelmRule `json:"helmRule,omitempty"`

	// HelmfileRule specifies the location of the helmfile to promote into
	HelmfileRule *HelmfileRule `json:"helmfileRule,omitempty"`

	// KptRule specifies to fetch the apps resource via kpt : https://googlecontainertools.github.io/kpt/
	KptRule *KptRule `json:"kptRule,omitempty"`
}

// HelmRule specifies which chart to add the app to the Chart's 'requirements.yaml' file
type HelmRule struct {
	// Path to the chart folder (which should contain Chart.yaml and requirements.yaml)
	Path string `json:"path"`
}

// HelmfileRule specifies which 'helmfile.yaml' file to use to promote the app into
type HelmfileRule struct {
	// Path to the helmfile to modify
	Path string `json:"path"`

	// Namespace if specified the given namespace is used in the `helmfile.yml` file when using Environments in the
	// same cluster using the same git repository URL as the dev environment
	Namespace string `json:"namespace"`

	// KeepOldVersions if specified is a list of release names and if the release name is in this list then the old versions are kept
	KeepOldVersions []string `json:"keepOldVersions"`
}

// KptRule specifies to fetch the apps resource via kpt : https://googlecontainertools.github.io/kpt/
type KptRule struct {
	// Path specifies the folder to fetch kpt resources into.
	// For example if the 'config-root'' directory contains a Config Sync git layout we may want applications to be deployed into the
	// `config-root/namespaces/myapps` folder. If so set the path to `config-root/namespaces/myapps`
	Path string `json:"path,omitempty"`

	// Namespace specifies the namespace to deploy applications if using kpt. If specified this value will be used instead
	// of the Environment.Spec.Namespace in the Environment CRD
	Namespace string `json:"namespace,omitempty"`
}

// FileRule specifies how to modify a 'Makefile` or shell script to add a new helm/kpt style command
type FileRule struct {
	// Path the path to the Makefile or shell script to modify. This is mandatory
	Path string `json:"path"`

	// LinePrefix adds a prefix to lines. e.g. for a Makefile that is typically "\t"
	LinePrefix string `json:"linePrefix,omitempty"`

	// InsertAfter finds the last line to match against to find where to insert
	InsertAfter []LineMatcher `json:"insertAfter,omitempty"`

	// UpdateTemplate matches line to perform upgrades to an app
	UpdateTemplate *LineMatcher `json:"updateTemplate,omitempty"`

	// CommandTemplate the command template for the promote command
	CommandTemplate string `json:"commandTemplate,omitempty"`
}

// LineMatcher specifies a rule on how to find a line to match
type LineMatcher struct {
	// Prefix the prefix of a line to match
	Prefix string `json:"prefix,omitempty"`

	// Regex the regex of a line to match
	Regex string `json:"regex,omitempty"`
}

// PromoteList contains a list of Promote
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PromoteList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Promote `json:"items"`
}
