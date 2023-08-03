package v1alpha1

import (
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	autoscalev2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// druid-operator deploys a druid cluster from given spec below, based on the spec it would create following
// k8s resources
// - one ConfigMap containing common.runtime.properties
// - for each item in the "nodes" field in spec
//   - one StatefulSet that manages one or more Druid pods with same config
//   - one ConfigMap containing runtime.properties, jvm.config, log4j.xml contents to be used by above Pods
//   - zero or more Headless/ClusterIP/LoadBalancer etc Service resources backed by above Pods
//   - optional PodDisruptionBudget resource for the StatefulSet
//

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AdditionalContainer defines the additional sidecar container
type AdditionalContainer struct {
	// List of configurations to use which are not present or to override default implementation configurations

	// This is the image for the additional container to run.
	// +required
	Image string `json:"image"`

	// This is the name of the additional container.
	// +required
	ContainerName string `json:"containerName"`

	// This is the command for the additional container to run.
	// +required
	Command []string `json:"command"`

	// If not present, will be taken from top level spec
	// +optional
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Argument to call the command
	// +optional
	Args []string `json:"args,omitempty"`

	// ContainerSecurityContext. If not present, will be taken from top level pod
	// +optional
	ContainerSecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`

	// CPU/Memory Resources
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`

	// Volumes etc for the Druid pods
	// +optional
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`

	// Environment variables for the Additional Container
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`

	// Extra environment variables
	// +optional
	EnvFrom []v1.EnvFromSource `json:"envFrom,omitempty"`
}

// DruidSpec defines the desired state of Druid
type DruidSpec struct {

	// Ignored is now deprecated API. In order to avoid reconciliation of objects use the
	// druid.apache.org/ignored: "true" annotation
	// +optional
	// +kubebuilder:default:=false
	Ignored bool `json:"ignored,omitempty"`

	// common.runtime.properties contents
	// +required
	CommonRuntimeProperties string `json:"common.runtime.properties"`

	// Optional: Default is true, will delete the sts pod if sts is set to ordered ready to ensure
	// issue: https://github.com/kubernetes/kubernetes/issues/67250
	// doc: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#forced-rollback

	// +optional
	ForceDeleteStsPodOnError bool `json:"forceDeleteStsPodOnError,omitempty"`

	// ScalePvcSts, defaults to false. When enabled, operator will allow volume expansion of sts and pvc's.
	// +optional
	ScalePvcSts bool `json:"scalePvcSts,omitempty"`

	// In-container directory to mount with common.runtime.properties
	// +required
	CommonConfigMountPath string `json:"commonConfigMountPath"`

	// Default is set to false, pvc shall be deleted on deletion of CR
	// +optional
	DisablePVCDeletionFinalizer bool `json:"disablePVCDeletionFinalizer,omitempty"`

	// Default is set to true, orphaned ( unmounted pvc's ) shall be cleaned up by the operator.
	// +optional
	DeleteOrphanPvc bool `json:"deleteOrphanPvc"`

	// Path to druid start script to be run on container start
	// +required
	StartScript string `json:"startScript"`

	// Optional: bash/sh entry arg. Set startScript to `sh` or `bash` to customize entryArg
	// For example, the container can run `sh -c "${EntryArg} && ${DruidScript} {nodeType}"`
	EntryArg string `json:"entryArg,omitempty"`

	// Optional: Customized druid shell script path. If not set, the default would be "bin/run-druid.sh"
	DruidScript string `json:"druidScript,omitempty"`

	// Required here or at nodeSpec level
	// +optional
	Image string `json:"image,omitempty"`

	// ServiceAccount for the druid cluster
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// imagePullSecrets for private registries
	// +optional
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// +optional
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Environment variables for druid containers
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`

	// Extra environment variables
	// +optional
	EnvFrom []v1.EnvFromSource `json:"envFrom,omitempty"`

	// jvm options for druid jvm processes
	// +optional
	JvmOptions string `json:"jvm.options,omitempty"`

	// log4j config contents
	// +optional
	Log4jConfig string `json:"log4j.config,omitempty"`

	// druid pods pod-security-context
	// +optional
	PodSecurityContext *v1.PodSecurityContext `json:"securityContext,omitempty"`

	// druid pods container-security-context
	// +optional
	ContainerSecurityContext *v1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// volumes etc for the Druid pods
	// +optional
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	// +optional
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// +optional
	Volumes []v1.Volume `json:"volumes,omitempty"`

	// Custom annotations to be populated in Druid pods
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// By default, it is set to "parallel"
	// +optional
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

	// Custom labels to be populated in Druid pods
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// +optional
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// Port is set to druid.port if not specified with httpGet handler
	// +optional
	LivenessProbe *v1.Probe `json:"livenessProbe,omitempty"`

	// Port is set to druid.port if not specified with httpGet handler
	// +optional
	ReadinessProbe *v1.Probe `json:"readinessProbe,omitempty"`

	// StartupProbe for nodeSpec
	// +optional
	StartUpProbe *v1.Probe `json:"startUpProbe,omitempty"`

	// k8s service resources to be created for each Druid statefulsets
	// +optional
	Services []v1.Service `json:"services,omitempty"`

	// Node selector to be used by Druid statefulsets
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Toleration to be used in order to run Druid on nodes tainted
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// Affinity to be used to for enabling node, pod affinity and anti-affinity
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`

	// Spec used to create StatefulSet specs etc, Many of the fields above can be overridden at the specific
	// node spec level.
	// Key in following map can be arbitrary string that helps you identify resources(pods, statefulsets etc) for specific nodeSpec.
	// But, it is used in the k8s resource names, so it must be compliant with restrictions
	// placed on k8s resource names.
	// that is, it must match regex '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'

	// +required
	Nodes map[string]DruidNodeSpec `json:"nodes"`

	// Operator deploys the sidecar container based on these properties. Sidecar will be deployed for all the Druid pods.
	// +optional
	AdditionalContainer []AdditionalContainer `json:"additionalContainer,omitempty"`

	// Operator deploys above list of nodes in the Druid prescribed order of Historical, Overlord, MiddleManager,
	// Broker, Coordinator etc.
	// If set to true then operator checks the rollout status of previous version StateSets before updating next.
	// Used only for updates.

	// +optional
	RollingDeploy bool `json:"rollingDeploy,omitempty"`

	// futuristic stuff to make Druid dependency setup extensible from within Druid operator
	// ignore for now.
	// +optional
	Zookeeper *ZookeeperSpec `json:"zookeeper,omitempty"`
	// +optional
	MetadataStore *MetadataStoreSpec `json:"metadataStore,omitempty"`
	// +optional
	DeepStorage *DeepStorageSpec `json:"deepStorage,omitempty"`

	// Custom Dimension Map Path for statsd emitter
	// +optional
	DimensionsMapPath string `json:"metricDimensions.json,omitempty"`
}

// +kubebuilder:object:root=true
type DruidNodeSpec struct {
	// Druid node type
	// +required
	// +kubebuilder:validation:Enum:=historical;overlord;middleManager;indexer;broker;coordinator;router
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	NodeType string `json:"nodeType"`

	// set only for the historical nodetype
	DeploymentConfig *DeploymentConfig `json:"deploymentConfig,omitempty"`

	// Port used by Druid Process
	// +required
	DruidPort int32 `json:"druid.port"`

	// Defaults to statefulsets.
	// Note: volumeClaimTemplates are ignored when kind=Deployment
	// +optional
	Kind string `json:"kind,omitempty"`

	// +required
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`

	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// +optional
	PodDisruptionBudgetSpec *policyv1.PodDisruptionBudgetSpec `json:"podDisruptionBudgetSpec,omitempty"`

	// +required
	RuntimeProperties string `json:"runtime.properties"`

	// This overrides JvmOptions at top level
	// +optional
	JvmOptions string `json:"jvm.options,omitempty"`

	// This appends extra jvm options to JvmOptions field
	// +optional
	ExtraJvmOptions string `json:"extra.jvm.options,omitempty"`

	// This overrides Log4jConfig at top level
	// +optional
	Log4jConfig string `json:"log4j.config,omitempty"`

	// in-container directory to mount with runtime.properties, jvm.config, log4j2.xml files
	// +required
	NodeConfigMountPath string `json:"nodeConfigMountPath"`

	// Overrides services at top level
	// +optional
	Services []v1.Service `json:"services,omitempty"`

	// Toleration to be used in order to run Druid on nodes tainted
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// Affinity to be used to for enabling node, pod affinity and anti-affinity
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`

	// Node selector to be used by Druid statefulsets
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// Extra ports to be added to pod spec
	// +optional
	Ports []v1.ContainerPort `json:"ports,omitempty"`

	// Overrides image from top level, Required if no image specified at top level
	// +optional
	Image string `json:"image,omitempty"`

	// Overrides imagePullSecrets from top level
	// +optional
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Overrides imagePullPolicy from top level
	// +optional
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Extra environment variables
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`

	// Extra environment variables
	// +optional
	EnvFrom []v1.EnvFromSource `json:"envFrom,omitempty"`

	// CPU/Memory Resources
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`

	// Overrides securityContext at top level
	// +optional
	PodSecurityContext *v1.PodSecurityContext `json:"securityContext,omitempty"`

	// Druid pods container-security-context
	// +optional
	ContainerSecurityContext *v1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// Custom annotations to be populated in Druid pods
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// By default, it is set to "parallel"
	// +optional
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

	// maxSurge for deployment object, only applicable if kind=Deployment, by default set to 25%
	// +optional
	MaxSurge *int32 `json:"maxSurge,omitempty"`

	// maxUnavailable for deployment object, only applicable if kind=Deployment, by default set to 25%
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`

	// +optional
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// +optional
	LivenessProbe *v1.Probe `json:"livenessProbe,omitempty"`

	// +optional
	ReadinessProbe *v1.Probe `json:"readinessProbe,omitempty"`

	// StartupProbe for nodeSpec
	// +optional
	StartUpProbes *v1.Probe `json:"startUpProbes,omitempty"`

	// Ingress Annoatations to be populated in ingress spec
	// +optional
	IngressAnnotations map[string]string `json:"ingressAnnotations,omitempty"`

	// Ingress Spec
	// +optional
	Ingress *networkingv1.IngressSpec `json:"ingress,omitempty"`

	// +optional
	PersistentVolumeClaim []v1.PersistentVolumeClaim `json:"persistentVolumeClaim,omitempty"`

	// +optional
	Lifecycle *v1.Lifecycle `json:"lifecycle,omitempty"`

	// +optional
	HPAutoScaler *autoscalev2.HorizontalPodAutoscalerSpec `json:"hpAutoscaler,omitempty"`

	// +optional
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// +optional
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	// +optional
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// +optional
	Volumes []v1.Volume `json:"volumes,omitempty"`
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

type DeploymentConfig struct {
	Mode      string `json:"mode"`
	BatchSize int32  `json:"batchSize,omitempty"`
}

// These are valid conditions of a druid Node
const (
	// DruidClusterReady indicates the underlying druid objects is fully deployed
	// Underlying pods are able to service requests
	DruidClusterReady DruidNodeConditionType = "DruidClusterReady"
	// DruidNodeRollingUpgrade means that Druid Node is rolling update.
	DruidNodeRollingUpdate DruidNodeConditionType = "DruidNodeRollingUpdate"
	// DruidNodeError indicates the DruidNode is in an error state.
	DruidNodeErrorState DruidNodeConditionType = "DruidNodeErrorState"
)

type DruidNodeConditionType string

type DruidNodeTypeStatus struct {
	DruidNode                string                 `json:"druidNode,omitempty"`
	DruidNodeConditionStatus v1.ConditionStatus     `json:"druidNodeConditionStatus,omitempty"`
	DruidNodeConditionType   DruidNodeConditionType `json:"druidNodeConditionType,omitempty"`
	Reason                   string                 `json:"reason,omitempty"`
}

type HistoricalStatus struct {
	Replica            int32    `json:"replica"`
	CurrentBatch       int32    `json:"currentBatch"`
	DecommissionedPods []string `json:"decommissionedPods"`
}

// DruidStatus defines the observed state of Druid
type DruidClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	DruidNodeStatus        DruidNodeTypeStatus `json:"druidNodeStatus,omitempty"`
	StatefulSets           []string            `json:"statefulSets,omitempty"`
	Deployments            []string            `json:"deployments,omitempty"`
	Services               []string            `json:"services,omitempty"`
	ConfigMaps             []string            `json:"configMaps,omitempty"`
	PodDisruptionBudgets   []string            `json:"podDisruptionBudgets,omitempty"`
	Ingress                []string            `json:"ingress,omitempty"`
	HPAutoScalers          []string            `json:"hpAutoscalers,omitempty"`
	Pods                   []string            `json:"pods,omitempty"`
	PersistentVolumeClaims []string            `json:"persistentVolumeClaims,omitempty"`
	Historical             HistoricalStatus    `json:"HistoricalStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// Druid is the Schema for the druids API
type Druid struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DruidSpec          `json:"spec"`
	Status DruidClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DruidList contains a list of Druid
type DruidList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Druid `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Druid{}, &DruidList{}, &DruidNodeSpec{})
}
