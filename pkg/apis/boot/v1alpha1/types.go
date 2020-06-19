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
	// KptPath if using kpt to deploy applications into a GitOps repository specify the folder to deploy into.
	// For example if the root directory contains a Config Sync git layout we may want applications to be deployed into the
	// `namespaces/myapps` folder. If the `myconfig` folder is used as the root of the Config Sync configuration you may want
	// to configure something like `myconfig/namespaces/mysystem` or whatever.
	KptPath string `json:"kptPath,omitempty"`

	// Namespace specifies the namespace to deploy applications if using kpt. If specified this value will be used instead
	// of the Environment.Spec.Namespace in the Environment CRD
	Namespace string `json:"namespace,omitempty"`
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
