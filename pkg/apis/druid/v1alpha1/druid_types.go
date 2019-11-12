package v1alpha1

import (
	"encoding/json"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DruidSpec defines the desired state of Druid
// +k8s:openapi-gen=true
//type DruidSpec struct {
// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
//}

// DruidStatus defines the observed state of Druid
// +k8s:openapi-gen=true
//type DruidStatus struct {
// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
//}

// druid-operator deploys a druid cluster from given spec below, based on the spec it would create following
// k8s resources
// - one ConfigMap containing common.runtime.properties
// - for each item in the "nodes" field in spec
//   - one StatefulSet that manages one or more Druid pods with same config)
//   - one ConfigMap containing runtime.properties, jvm.config, log4j.xml contents to be used by above Pods
//   - zero or more Headless/ClusterIP/LoadBalancer etc Service resources backed by above Pods
//   - optional PodDisruptionBudget resource for the StatefulSet
//
type DruidClusterSpec struct {
	// Optional: If true, this spec would be ignored by the operator
	Ignored bool `json:"ignored,omitempty"`

	// Required: common.runtime.properties contents
	CommonRuntimeProperties string `json:"common.runtime.properties"`

	// Required: in-container directory to mount with common.runtime.properties
	CommonConfigMountPath string `json:"commonConfigMountPath"`

	// Required: path to druid start script to be run on container start
	StartScript string `json:"startScript"`

	// Required: Druid Docker Image
	Image string `json:"image"`

	// Optional: environment variables for druid containers
	Env []v1.EnvVar `json:"env,omitempty"`

	// Optional: jvm options for druid jvm processes
	JvmOptions string `json:"jvm.options,omitempty"`

	// Optional: log4j config contents
	Log4jConfig string `json:"log4j.config,omitempty"`

	// Optional: druid pods security-context
	SecurityContext *v1.PodSecurityContext `json:"securityContext,omitempty"`

	// Optional: volumes etc for the Druid pods
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	VolumeMounts         []v1.VolumeMount           `json:"volumeMounts,omitempty"`
	Volumes              []v1.Volume                `json:"volumes,omitempty"`

	// Optional: custom annotations to be populated in Druid pods
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// Optional: By default it is set to "parallel"
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

	// Optional
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// Optional, port is set to druid.port if not specified with httpGet handler
	LivenessProbe *v1.Probe `json:"livenessProbe,omitempty"`

	// Optional, port is set to druid.port if not specified with httpGet handler
	ReadinessProbe *v1.Probe `json:"readinessProbe,omitempty"`

	// Optional: k8s service resources to be created for each Druid statefulsets
	Services []v1.Service `json:"services,omitempty"`

	// Optional: node selector to be used by Druid statefulsets
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Spec used to create StatefulSet specs etc, Many of the fields above can be overridden at the specific
	// node spec level.

	// Key in following map can be arbitrary string that helps you identify resources(pods, statefulsets etc) for specific nodeSpec.
	// But, it is used in the k8s resource names, so it must be compliant with restrictions
	// placed on k8s resource names.
	// that is, it must match regex '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'
	Nodes map[string]DruidNodeSpec `json:"nodes"`

	// Operator deploys above list of nodes in the Druid prescribed order of Historical, Overlord, MiddleManager,
	// Broker, Coordinator etc.
	// Optional: If set to true then operator checks the rollout status of previous version StateSets before updating next.
	// Used only for updates.
	RollingDeploy bool `json:"rollingDeploy,omitempty"`

	// futuristic stuff to make Druid dependency setup extensible from within Druid operator
	// ignore for now.
	Zookeeper     *ZookeeperSpec     `json:"zookeeper"`
	MetadataStore *MetadataStoreSpec `json:"metadataStore"`
	DeepStorage   *DeepStorageSpec   `json:"deepStorage"`
}

type DruidNodeSpec struct {
	// Required: Druid node type e.g. Broker, Coordinator, Historical, MiddleManager, Router, Overlord etc
	NodeType string `json:"nodeType"`

	// Required: Port used by Druid Process
	DruidPort int32 `json:"druid.port"`

	// Required
	Replicas int32 `json:"replicas"`

	// Optional
	PodDisruptionBudgetSpec *v1beta1.PodDisruptionBudgetSpec `json:"podDisruptionBudgetSpec"`

	// Required
	RuntimeProperties string `json:"runtime.properties"`

	// Optional: This overrides JvmOptions at top level
	JvmOptions string `json:"jvm.options,omitempty"`

	// Optional: This appends extra jvm options to JvmOptions field
	ExtraJvmOptions string `json:"extra.jvm.options,omitempty"`

	// Optional: This overrides Log4jConfig at top level
	Log4jConfig string `json:"log4j.config,omitempty"`

	// Required: in-container directory to mount with runtime.properties, jvm.config, log4j2.xml files
	NodeConfigMountPath string `json:"nodeConfigMountPath,omitempty"`

	// Optional: Overrides services at top level
	Services []v1.Service `json:"services,omitempty"`

	// Optional: extra ports to be added to pod spec
	Ports []v1.ContainerPort `json:"ports,omitempty"`

	// Optional: Overrides image from top level
	Image string `json:"image,omitempty"`

	// Optional: Extra environment variables
	Env []v1.EnvVar `json:"env,omitempty"`

	// Optional: CPU/Memory Resources
	Resources v1.ResourceRequirements `json:"resources,omitempty"`

	// Optional: Overrides securityContext at top level
	SecurityContext *v1.PodSecurityContext `json:"securityContext,omitempty"`

	// Optional: custom annotations to be populated in Druid pods
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// Optional: By default it is set to "parallel"
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

	// Optional
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// Optional
	LivenessProbe *v1.Probe `json:"livenessProbe,omitempty"`

	// Optional
	ReadinessProbe *v1.Probe `json:"readinessProbe,omitempty"`

	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	VolumeMounts         []v1.VolumeMount           `json:"volumeMounts,omitempty"`
	Volumes              []v1.Volume                `json:"volumes,omitempty"`
}

type ZookeeperSpec struct {
	Type string          `json:"type"`
	Spec json.RawMessage `json:"spec"`
}

type MetadataStoreSpec struct {
	Type string          `json:"type"`
	Spec json.RawMessage `json:"spec"`
}

type DeepStorageSpec struct {
	Type string          `json:"type"`
	Spec json.RawMessage `json:"spec"`
}

type DruidClusterStatus struct {
	StatefulSets         []string `json:"statefulSets,omitempty"`
	Services             []string `json:"services,omitempty"`
	ConfigMaps           []string `json:"configMaps,omitempty"`
	PodDisruptionBudgets []string `json:"podDisruptionBudgets,omitempty"`
	Pods                 []string `json:"pods,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Druid is the Schema for the druids API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=druids,scope=Namespaced
type Druid struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DruidClusterSpec   `json:"spec,omitempty"`
	Status DruidClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DruidList contains a list of Druid
type DruidList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Druid `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Druid{}, &DruidList{})
}
