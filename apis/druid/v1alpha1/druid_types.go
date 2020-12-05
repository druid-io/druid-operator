/*

 */

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DruidSpec defines the desired state of Druid
type DruidSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Druid. Edit Druid_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// DruidStatus defines the observed state of Druid
type DruidStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// Druid is the Schema for the druids API
type Druid struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DruidSpec   `json:"spec,omitempty"`
	Status DruidStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DruidList contains a list of Druid
type DruidList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Druid `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Druid{}, &DruidList{})
}
