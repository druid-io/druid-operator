package v1alpha1

import (
	"encoding/json"

	"k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DruidClusterSpec struct {
	Image                   string `json:"image"`
	CommonRuntimeProperties string `json:"common.runtime.properties"`

	Env                  []v1.EnvVar                `json:"env,omitempty"`
	JvmOptions           string                     `json:"jvm.options,omitempty"`
	Log4jConfig          string                     `json:"log4j.config,omitempty"`
	SecurityContext      *v1.PodSecurityContext     `json:"securityContext,omitempty"`
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	VolumeMounts         []v1.VolumeMount           `json:"volumeMounts,omitempty"`
	Volumes              []v1.Volume                `json:"volumes,omitempty"`
	PodAnnotations       map[string]string          `json:"podAnnotations,omitempty"`
	//Common service port to expose druid API on LB
	ServicePort int `json:"servicePort,omitempty"`

	// Key can be arbitrary string that helps you identify resources(pods, statefulsets etc) for specific nodeSpec.
	// But, it is used in the resource names, so it must be compliant with restrictions
	// placed on k8s resource names.
	// that is, it must match regex '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'
	Nodes map[string]DruidNodeSpec `json:"nodes"`

	Zookeeper     *ZookeeperSpec     `json:"zookeeper"`
	MetadataStore *MetadataStoreSpec `json:"metadataStore"`
	DeepStorage   *DeepStorageSpec   `json:"deepStorage"`
	NodeSelector  map[string]string  `json:"nodeSelector,omitempty"`
}

type DruidNodeSpec struct {
	NodeType                string                           `json:"nodeType"`
	DruidPort               int32                            `json:"druid.port"`
	Replicas                int32                            `json:"replicas"`
	PodDisruptionBudgetSpec *v1beta1.PodDisruptionBudgetSpec `json:"podDisruptionBudgetSpec"`
	RuntimeProperties       string                           `json:"runtime.properties"`
	JvmOptions              string                           `json:"jvm.options,omitempty"`
	ExtraJvmOptions         string                           `json:"extra.jvm.options,omitempty"`
	Log4jConfig             string                           `json:"log4j.config,omitempty"`

	Service              *v1.Service                `json:"service,omitempty"`
	Ports                []v1.ContainerPort         `json:"ports,omitempty"`
	Image                string                     `json:"image,omitempty"`
	Env                  []v1.EnvVar                `json:"env,omitempty"`
	Resources            v1.ResourceRequirements    `json:"resources,omitempty"`
	SecurityContext      *v1.PodSecurityContext     `json:"securityContext,omitempty"`
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	VolumeMounts         []v1.VolumeMount           `json:"volumeMounts,omitempty"`
	Volumes              []v1.Volume                `json:"volumes,omitempty"`
}

type ZookeeperSpec struct {
	Type string
	Spec json.RawMessage
}

type MetadataStoreSpec struct {
	Type string
	Spec json.RawMessage
}

type DeepStorageSpec struct {
	Type string
	Spec json.RawMessage
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DruidList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Druid `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Druid struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              DruidClusterSpec   `json:"spec"`
	Status            DruidClusterStatus `json:"status,omitempty" patchStrategy:"merge"`
}

type DruidClusterStatus struct {
	StatefulSets         []string `json:"statefulSets,omitempty"`
	Services             []string `json:"services,omitempty"`
	ConfigMaps           []string `json:"configMaps,omitempty"`
	PodDisruptionBudgets []string `json:"podDisruptionBudgets,omitempty"`
	Pods                 []string `json:"pods,omitempty"`
}
