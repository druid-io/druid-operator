package druid

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/druid-io/druid-operator/pkg/apis/druid/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"regexp"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	druidOpResourceHash = "druidOpResourceHash"
	broker              = "broker"
	coordinator         = "coordinator"
	overlord            = "overlord"
	middleManager       = "middleManager"
	historical          = "historical"
	router              = "router"
)

var logger = logf.Log.WithName("druid_operator_handler")

func deployDruidCluster(sdk client.Client, m *v1alpha1.Druid) error {
	if m.Spec.Ignored {
		return nil
	}

	if err := verifyDruidSpec(m); err != nil {
		e := fmt.Errorf("Invalid DruidSpec[%s:%s] due to [%s].", m.Kind, m.Name, err.Error())
		sendEvent(sdk, m, v1.EventTypeWarning, "SPEC_INVALID", e.Error())
		return nil
	}

	allNodeSpecs, err := getAllNodeSpecsInDruidPrescribedOrder(m)
	if err != nil {
		e := fmt.Errorf("Invalid DruidSpec[%s:%s] due to [%s].", m.Kind, m.Name, err.Error())
		sendEvent(sdk, m, v1.EventTypeWarning, "SPEC_INVALID", e.Error())
		return nil
	}

	statefulSetNames := make(map[string]bool)
	serviceNames := make(map[string]bool)
	configMapNames := make(map[string]bool)
	podDisruptionBudgetNames := make(map[string]bool)

	ls := makeLabelsForDruid(m.Name)

	commonConfig, err := makeCommonConfigMap(m, ls)
	if err != nil {
		return err
	}
	commonConfigSHA, err := getObjectHash(commonConfig)
	if err != nil {
		return err
	}

	if err := sdkCreateOrUpdateAsNeeded(sdk,
		func() (object, error) { return makeCommonConfigMap(m, ls) },
		func() object { return makeConfigMapEmptyObj() },
		nil, m, configMapNames); err != nil {
		return err
	}

	for _, elem := range allNodeSpecs {
		key := elem.key
		nodeSpec := elem.spec

		//Name in k8s must pass regex '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'
		//So this unique string must follow same.
		nodeSpecUniqueStr := makeNodeSpecificUniqueString(m, key)

		lm := makeLabelsForNodeSpec(m.Name, nodeSpecUniqueStr)

		// create configmap first
		nodeConfig, err := makeConfigMapForNodeSpec(&nodeSpec, m, lm, nodeSpecUniqueStr)
		if err != nil {
			return err
		}

		nodeConfigSHA, err := getObjectHash(nodeConfig)
		if err != nil {
			return err
		}

		if err := sdkCreateOrUpdateAsNeeded(sdk,
			func() (object, error) { return nodeConfig, nil },
			func() object { return makeConfigMapEmptyObj() },
			nil, m, configMapNames); err != nil {
			return err
		}

		//create service before creating statefulset
		if nodeSpec.Service != nil {
			if err := sdkCreateOrUpdateAsNeeded(sdk,
				func() (object, error) { return makeLoadBalancerService(&nodeSpec, m, lm, nodeSpecUniqueStr) },
				func() object { return makeServiceEmptyObj() },
				func(prev, curr object) { (curr.(*v1.Service)).Spec.ClusterIP = (prev.(*v1.Service)).Spec.ClusterIP },
				m, serviceNames); err != nil {
				return err
			}
		}

		// one more headless service required for stateful
		if err := sdkCreateOrUpdateAsNeeded(sdk,
			func() (object, error) {
				return makeHeadlessService(fmt.Sprintf("%s", nodeSpecUniqueStr), m.Namespace, lm, nodeSpec.DruidPort)
			},
			func() object { return makeServiceEmptyObj() },
			nil, m, serviceNames); err != nil {
			return err
		}

		nodeSpec.Ports = append(nodeSpec.Ports, v1.ContainerPort{ContainerPort: nodeSpec.DruidPort, Name: "druid-port"})

		// Create StatefulSet
		if err := sdkCreateOrUpdateAsNeeded(sdk,
			func() (object, error) {
				return makeStatefulSet(&nodeSpec, m, lm, nodeSpecUniqueStr, fmt.Sprintf("%s-%s", commonConfigSHA, nodeConfigSHA))
			},
			func() object { return makeStatefulSetEmptyObj() },
			nil, m, statefulSetNames); err != nil {
			return err
		}

		// Create PodDisruptionBudget
		if nodeSpec.PodDisruptionBudgetSpec != nil {
			if err := sdkCreateOrUpdateAsNeeded(sdk,
				func() (object, error) { return makePodDisruptionBudget(&nodeSpec, m, lm, nodeSpecUniqueStr) },
				func() object { return makePodDisruptionBudgetEmptyObj() },
				nil, m, podDisruptionBudgetNames); err != nil {
				return err
			}
		}
	}

	//update status and delete unwanted resources
	updatedStatus := v1alpha1.DruidClusterStatus{}

	updatedStatus.StatefulSets = deleteUnusedResources(sdk, m, statefulSetNames, ls,
		func() runtime.Object { return makeStatefulSetListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*appsv1.StatefulSetList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		})
	sort.Strings(updatedStatus.StatefulSets)

	updatedStatus.PodDisruptionBudgets = deleteUnusedResources(sdk, m, podDisruptionBudgetNames, ls,
		func() runtime.Object { return makePodDisruptionBudgetListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*v1beta1.PodDisruptionBudgetList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		})
	sort.Strings(updatedStatus.PodDisruptionBudgets)

	updatedStatus.Services = deleteUnusedResources(sdk, m, serviceNames, ls,
		func() runtime.Object { return makeServiceListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*v1.ServiceList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		})
	sort.Strings(updatedStatus.Services)

	updatedStatus.ConfigMaps = deleteUnusedResources(sdk, m, configMapNames, ls,
		func() runtime.Object { return makeConfigMapListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*v1.ConfigMapList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		})
	sort.Strings(updatedStatus.ConfigMaps)

	podList := podList()
	listOpts := []client.ListOption{
		client.InNamespace(m.Namespace),
		client.MatchingLabels(makeLabelsForDruid(m.Name)),
	}

	if err := sdk.List(context.TODO(), podList, listOpts...); err != nil {
		e := fmt.Errorf("Failed to list pods for [%s:%s] due to [%s].", m.Kind, m.Name, err.Error())
		sendEvent(sdk, m, v1.EventTypeWarning, "LIST_FAIL", e.Error())
		logger.Error(e, e.Error(), "name", m.Name, "namespace", m.Namespace)
	}
	updatedStatus.Pods = getPodNames(podList.Items)
	sort.Strings(updatedStatus.Pods)

	if !reflect.DeepEqual(updatedStatus, m.Status) {
		patchBytes, err := json.Marshal(map[string]v1alpha1.DruidClusterStatus{"status": updatedStatus})
		if err != nil {
			return fmt.Errorf("failed to serialize status patch to bytes: %v", err)
		}
		if err := sdk.Patch(context.TODO(), m, client.ConstantPatch(types.MergePatchType, patchBytes)); err != nil {
			e := fmt.Errorf("Failed to update status for [%s:%s] due to [%s].", m.Kind, m.Name, err.Error())
			sendEvent(sdk, m, v1.EventTypeWarning, "UPDATE_FAIL", e.Error())
			logger.Error(e, e.Error(), "name", m.Name, "namespace", m.Namespace)
		}
	}

	return nil
}

func deleteUnusedResources(sdk client.Client, drd *v1alpha1.Druid,
	names map[string]bool, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, itemsExtractorFn func(obj runtime.Object) []object) []string {

	listOpts := []client.ListOption{
		client.InNamespace(drd.Namespace),
		client.MatchingLabels(selectorLabels),
	}

	survivorNames := make([]string, 0, len(names))

	listObj := emptyListObjFn()
	if err := sdk.List(context.TODO(), listObj, listOpts...); err != nil {
		e := fmt.Errorf("Failed to list [%s] due to [%s].", listObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		sendEvent(sdk, drd, v1.EventTypeWarning, "LIST_FAIL", e.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
	} else {
		for _, s := range itemsExtractorFn(listObj) {
			if names[s.GetName()] == false {
				if err := sdkDelete(sdk, context.TODO(), s); err != nil {
					e := fmt.Errorf("Failed to delete [%s:%s] due to [%s].", listObj.GetObjectKind().GroupVersionKind().Kind, s.GetName(), err.Error())
					sendEvent(sdk, drd, v1.EventTypeWarning, "DELETE_FAIL", e.Error())
					logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
					survivorNames = append(survivorNames, s.GetName())
				} else {
					sendEvent(sdk, drd, v1.EventTypeNormal, "DELETE_SUCCESS", fmt.Sprintf("Deleted [%s:%s].", listObj.GetObjectKind().GroupVersionKind().Kind, s.GetName()))
				}
			} else {
				survivorNames = append(survivorNames, s.GetName())
			}
		}
	}

	return survivorNames
}

type object interface {
	metav1.Object
	runtime.Object
}

func sdkCreateOrUpdateAsNeeded(sdk client.Client, objFn func() (object, error), emptyObjFn func() object, updaterFn func(prev, curr object), drd *v1alpha1.Druid, names map[string]bool) error {
	if obj, err := objFn(); err != nil {
		return err
	} else {
		names[obj.GetName()] = true

		addOwnerRefToObject(obj, asOwner(drd))
		addHashToObject(obj)

		if err := sdkCreate(sdk, context.TODO(), obj); err != nil {
			if apierrors.IsAlreadyExists(err) {
				prevObj := emptyObjFn()
				if err := sdk.Get(context.TODO(), *namespacedName(obj.GetName(), obj.GetNamespace()), prevObj); err != nil {
					e := fmt.Errorf("Failed to get [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
					logger.Error(e, e.Error(), "Prev object", stringifyForLogging(prevObj, drd), "name", drd.Name, "namespace", drd.Namespace)
					sendEvent(sdk, drd, v1.EventTypeWarning, "UPDATE_FAIL", e.Error())
				} else {
					if obj.GetAnnotations()[druidOpResourceHash] != prevObj.GetAnnotations()[druidOpResourceHash] {

						obj.SetResourceVersion(prevObj.GetResourceVersion())
						if updaterFn != nil {
							updaterFn(prevObj, obj)
						}

						if err := sdk.Update(context.TODO(), obj); err != nil {
							e := fmt.Errorf("Failed to update [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
							logger.Error(e, e.Error(), "Current Object", stringifyForLogging(prevObj, drd), "Updated Object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
							sendEvent(sdk, drd, v1.EventTypeWarning, "UPDATE_FAIL", e.Error())
						} else {
							msg := fmt.Sprintf("Updated [%s:%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
							logger.Info(msg, "Prev Object", stringifyForLogging(prevObj, drd), "Updated Object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
							sendEvent(sdk, drd, v1.EventTypeNormal, "UPDATE_SUCCESS", msg)
						}
					}
				}
			} else {
				e := fmt.Errorf("Failed to create [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
				logger.Error(e, e.Error(), "object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
				sendEvent(sdk, drd, v1.EventTypeWarning, "CREATE_FAIL", e.Error())
			}
		} else {
			msg := fmt.Sprintf("Created [%s:%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
			logger.Info(msg, "Object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
			sendEvent(sdk, drd, v1.EventTypeNormal, "CREATE_SUCCESS", msg)
		}
	}

	return nil
}

func stringifyForLogging(obj object, drd *v1alpha1.Druid) string {
	if bytes, err := json.Marshal(obj); err != nil {
		logger.Error(err, err.Error(), fmt.Sprintf("Failed to serialize [%s:%s]", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()), "name", drd.Name, "namespace", drd.Namespace)
		return fmt.Sprintf("%v", obj)
	} else {
		return string(bytes)
	}

}

func addHashToObject(obj object) error {
	if sha, err := getObjectHash(obj); err != nil {
		return err
	} else {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
			obj.SetAnnotations(annotations)
		}
		annotations[druidOpResourceHash] = sha
		return nil
	}
}

func getObjectHash(obj object) (string, error) {
	if bytes, err := json.Marshal(obj); err != nil {
		return "", err
	} else {
		sha1Bytes := sha1.Sum(bytes)
		return base64.StdEncoding.EncodeToString(sha1Bytes[:]), nil
	}
}

func makeNodeSpecificUniqueString(m *v1alpha1.Druid, key string) string {
	return fmt.Sprintf("druid-%s-%s", m.Name, key)
}

func makeCommonConfigMap(m *v1alpha1.Druid, ls map[string]string) (*v1.ConfigMap, error) {
	prop := m.Spec.CommonRuntimeProperties

	if m.Spec.Zookeeper != nil {
		if zm, err := createZookeeperManager(m.Spec.Zookeeper); err != nil {
			return nil, err
		} else {
			prop = prop + "\n" + zm.Configuration() + "\n"
		}
	}

	if m.Spec.MetadataStore != nil {
		if msm, err := createMetadataStoreManager(m.Spec.MetadataStore); err != nil {
			return nil, err
		} else {
			prop = prop + "\n" + msm.Configuration() + "\n"
		}
	}

	if m.Spec.DeepStorage != nil {
		if dsm, err := createDeepStorageManager(m.Spec.DeepStorage); err != nil {
			return nil, err
		} else {
			prop = prop + "\n" + dsm.Configuration() + "\n"
		}
	}

	cfg, err := makeConfigMap(
		fmt.Sprintf("%s-druid-common-config", m.ObjectMeta.Name),
		m.Namespace,
		ls,
		map[string]string{"common.runtime.properties": prop})
	return cfg, err
}

func makeConfigMapForNodeSpec(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, lm map[string]string, nodeSpecUniqueStr string) (*v1.ConfigMap, error) {
	return makeConfigMap(
		fmt.Sprintf("%s-config", nodeSpecUniqueStr),
		m.Namespace,
		lm,
		map[string]string{
			"runtime.properties": fmt.Sprintf("druid.port=%d\n%s", nodeSpec.DruidPort, nodeSpec.RuntimeProperties),
			"jvm.config":         fmt.Sprintf("%s\n%s", firstNonEmptyStr(nodeSpec.JvmOptions, m.Spec.JvmOptions), nodeSpec.ExtraJvmOptions),
			"log4j2.xml":         firstNonEmptyStr(nodeSpec.Log4jConfig, m.Spec.Log4jConfig)})
}

func makeConfigMap(name string, namespace string, labels map[string]string, data map[string]string) (*v1.ConfigMap, error) {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}, nil
}

func makeLoadBalancerService(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr string) (*v1.Service, error) {
	svc := nodeSpec.Service

	svc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	}

	if svc.ObjectMeta.Name == "" {
		svc.ObjectMeta.Name = fmt.Sprintf("%s-%s-service", nodeSpec.NodeType, nodeSpecUniqueStr)
	}

	if svc.ObjectMeta.Namespace == "" {
		svc.ObjectMeta.Namespace = m.Namespace
	}

	if svc.ObjectMeta.Labels == nil {
		svc.ObjectMeta.Labels = ls
	} else {
		for k, v := range ls {
			svc.ObjectMeta.Labels[k] = v
		}
	}
	var servicePort int32
	if m.Spec.ServicePort == 0 {
		servicePort = nodeSpec.DruidPort
	} else {
		servicePort = int32(m.Spec.ServicePort)
	}
	svc.Spec.Ports = []v1.ServicePort{
		{
			Name:       "service-port",
			Port:       servicePort,
			TargetPort: intstr.FromInt(int(nodeSpec.DruidPort)),
		},
	}

	if svc.Spec.Selector == nil {
		svc.Spec.Selector = ls
	} else {
		for k, v := range ls {
			svc.Spec.Selector[k] = v
		}
	}

	return svc, nil
}

func makeHeadlessService(name string, namespace string, labels map[string]string, port int32) (*v1.Service, error) {
	l := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name: "service-port",
					Port: port,
					TargetPort: intstr.FromInt(int(port)),
				},
			},
			Selector:  labels,
			Type:      "ClusterIP",
			ClusterIP: "None",
		},
	}

	return l, nil
}

func makeStatefulSet(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr, configMapSHA string) (*appsv1.StatefulSet, error) {
	templateHolder := []v1.PersistentVolumeClaim{}

	for _, val := range m.Spec.VolumeClaimTemplates {
		templateHolder = append(templateHolder, val)
	}

	for _, val := range nodeSpec.VolumeClaimTemplates {
		templateHolder = append(templateHolder, val)
	}

	volumeMountHolder := []v1.VolumeMount{
		{
			MountPath: "/druid/conf/druid/_common",
			Name:      "common-config-volume",
		},
		{
			MountPath: fmt.Sprintf("/druid/conf/druid/%s", nodeSpec.NodeType),
			Name:      "nodetype-config-volume",
		},
	}

	volumeMountHolder = append(volumeMountHolder, m.Spec.VolumeMounts...)
	volumeMountHolder = append(volumeMountHolder, nodeSpec.VolumeMounts...)

	volumesHolder := []v1.Volume{
		{
			Name: "common-config-volume",

			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: fmt.Sprintf("%s-druid-common-config", m.ObjectMeta.Name),
					},
				}},
		},
		{
			Name: "nodetype-config-volume",

			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: fmt.Sprintf("%s-config", nodeSpecUniqueStr),
					},
				},
			},
		},
	}
	volumesHolder = append(volumesHolder, m.Spec.Volumes...)
	volumesHolder = append(volumesHolder, nodeSpec.Volumes...)

	envHolder := firstNonNilValue(nodeSpec.Env, m.Spec.Env).([]v1.EnvVar)
	// enables to do the trick to force redeployment in case of configmap changes.
	envHolder = append(envHolder, v1.EnvVar{Name: "configMapSHA", Value: configMapSHA})

	// Create StatefulSet
	result := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s", nodeSpecUniqueStr),
			Namespace: m.Namespace,
			Labels:    ls,
		},

		Spec: appsv1.StatefulSetSpec{
			ServiceName:         fmt.Sprintf("%s", nodeSpecUniqueStr),
			Replicas:            &nodeSpec.Replicas,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      ls,
					Annotations: m.Spec.PodAnnotations,
				},
				Spec: v1.PodSpec{
					NodeSelector: m.Spec.NodeSelector,
					Containers: []v1.Container{
						{
							Image:        firstNonEmptyStr(nodeSpec.Image, m.Spec.Image),
							Name:         fmt.Sprintf("%s", nodeSpecUniqueStr),
							Command:      []string{"bin/run-druid.sh", nodeSpec.NodeType},
							Ports:        nodeSpec.Ports,
							Resources:    nodeSpec.Resources,
							Env:          envHolder,
							VolumeMounts: volumeMountHolder,
						},
					},
					Volumes:         volumesHolder,
					SecurityContext: firstNonNilValue(nodeSpec.SecurityContext, m.Spec.SecurityContext).(*v1.PodSecurityContext),
				},
			},
			VolumeClaimTemplates: templateHolder,
		},
	}

	return result, nil
}

func makePodDisruptionBudget(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr string) (*v1beta1.PodDisruptionBudget, error) {
	pdbSpec := *nodeSpec.PodDisruptionBudgetSpec
	pdbSpec.Selector = &metav1.LabelSelector{MatchLabels: ls}

	pdb := &v1beta1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policy/v1beta1",
			Kind:       "PodDisruptionBudget",
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeSpecUniqueStr,
			Namespace: m.Namespace,
			Labels:    ls,
		},

		Spec: pdbSpec,
	}

	return pdb, nil
}

// makeLabelsForDruid returns the labels for selecting the resources
// belonging to the given memcached CR name.
func makeLabelsForDruid(name string) map[string]string {
	return map[string]string{"app": "druid", "druid_cr": name}
}

// makeLabelsForDruid returns the labels for selecting the resources
// belonging to the given memcached CR name.
func makeLabelsForNodeSpec(clusterName, nodeSpecUniqueStr string) map[string]string {
	return map[string]string{"app": "druid", "druid_cr": clusterName, "nodeSpecUniqueStr": nodeSpecUniqueStr}
}

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}

// asOwner returns an OwnerReference set as the memcached CR
func asOwner(m *v1alpha1.Druid) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: m.APIVersion,
		Kind:       m.Kind,
		Name:       m.Name,
		UID:        m.UID,
		Controller: &trueVar,
	}
}

// podList returns a v1.PodList object
func podList() *v1.PodList {
	return &v1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}
}

func makeStatefulSetListEmptyObj() *appsv1.StatefulSetList {
	return &appsv1.StatefulSetList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
	}
}

func makePodDisruptionBudgetListEmptyObj() *v1beta1.PodDisruptionBudgetList {
	return &v1beta1.PodDisruptionBudgetList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policy/v1beta1",
			Kind:       "PodDisruptionBudget",
		},
	}
}

func makeConfigMapListEmptyObj() *v1.ConfigMapList {
	return &v1.ConfigMapList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
	}
}

func makeServiceListEmptyObj() *v1.ServiceList {
	return &v1.ServiceList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
	}
}

func makeStatefulSetEmptyObj() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
	}
}

func makePodDisruptionBudgetEmptyObj() *v1beta1.PodDisruptionBudget {
	return &v1beta1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policy/v1beta1",
			Kind:       "PodDisruptionBudget",
		},
	}
}

func makeServiceEmptyObj() *v1.Service {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
	}
}

func makeConfigMapEmptyObj() *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
	}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []v1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

func sendEvent(sdk client.Client, drd *v1alpha1.Druid, eventtype, reason, message string) {

	ref := &v1.ObjectReference{
		Kind:            drd.Kind,
		APIVersion:      drd.APIVersion,
		Name:            drd.Name,
		Namespace:       drd.Namespace,
		UID:             drd.UID,
		ResourceVersion: drd.ResourceVersion,
	}

	t := metav1.Now()
	namespace := ref.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}

	event := &v1.Event{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Event",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: namespace,
		},
		InvolvedObject: *ref,
		Reason:         reason,
		Message:        message,
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Type:           eventtype,
		Source:         v1.EventSource{Component: "druid-operator"},
	}

	if err := sdk.Create(context.TODO(), event); err != nil {
		logger.Error(err, fmt.Sprintf("Failed to push event [%v]", event))
	}
}

func verifyDruidSpec(drd *v1alpha1.Druid) error {
	keyValidationRegex, err := regexp.Compile("[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*")
	if err != nil {
		return err
	}

	for key, _ := range drd.Spec.Nodes {
		if !keyValidationRegex.MatchString(key) {
			return fmt.Errorf("Key[%s] must match k8s resource name regex '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'", key)
		}
	}

	return nil
}

type keyAndNodeSpec struct {
	key  string
	spec v1alpha1.DruidNodeSpec
}

// Recommended prescribed order is described at http://druid.io/docs/latest/operations/rolling-updates.html
func getAllNodeSpecsInDruidPrescribedOrder(m *v1alpha1.Druid) ([]keyAndNodeSpec, error) {
	nodeSpecsByNodeType := map[string][]keyAndNodeSpec{
		broker:        make([]keyAndNodeSpec, 0, 1),
		coordinator:   make([]keyAndNodeSpec, 0, 1),
		historical:    make([]keyAndNodeSpec, 0, 1),
		overlord:      make([]keyAndNodeSpec, 0, 1),
		middleManager: make([]keyAndNodeSpec, 0, 1),
		router:        make([]keyAndNodeSpec, 0, 1),
	}

	for key, nodeSpec := range m.Spec.Nodes {
		nodeSpecs := nodeSpecsByNodeType[nodeSpec.NodeType]
		if nodeSpecs == nil {
			return nil, fmt.Errorf("DruidSpec[%s:%s] has invalid NodeType[%s]. Deployment aborted.", m.Kind, m.Name, nodeSpec.NodeType)
		} else {
			nodeSpecsByNodeType[nodeSpec.NodeType] = append(nodeSpecs, keyAndNodeSpec{key, nodeSpec})
		}
	}

	allNodeSpecs := make([]keyAndNodeSpec, 0, len(m.Spec.Nodes))

	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[historical]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[overlord]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[middleManager]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[broker]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[coordinator]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[router]...)

	return allNodeSpecs, nil
}

func namespacedName(name, namespace string) *types.NamespacedName {
	return &types.NamespacedName{Name: name, Namespace: namespace}
}

//-------------------------------------------
// resetGroupVersionKind func is copied from controller-runtime/pkg/client/client.go to retain TypeMeta
// on sdk.Create/Delete , PATCH/UPDATE already retain that

// resetGroupVersionKind is a helper function to restore and preserve GroupVersionKind on an object.
// TODO(vincepri): Remove this function and its calls once    controller-runtime dependencies are upgraded to 1.15.
func resetGroupVersionKind(obj runtime.Object, gvk schema.GroupVersionKind) {
	if gvk != schema.EmptyObjectKind.GroupVersionKind() {
		if v, ok := obj.(schema.ObjectKind); ok {
			v.SetGroupVersionKind(gvk)
		}
	}
}

// Create implements client.Client
func sdkCreate(sdk client.Client, ctx context.Context, obj runtime.Object) error {
	defer resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())
	return sdk.Create(ctx, obj)
}

func sdkDelete(sdk client.Client, ctx context.Context, obj runtime.Object) error {
	defer resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())
	return sdk.Delete(ctx, obj)
}
//--------------------------------------------------------------------------------------------------------------------
