package druid

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"

	autoscalev2beta2 "k8s.io/api/autoscaling/v2beta2"
	networkingv1 "k8s.io/api/networking/v1"
	storage "k8s.io/api/storage/v1"

	"github.com/druid-io/druid-operator/apis/druid/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	druidOpResourceHash          = "druidOpResourceHash"
	broker                       = "broker"
	coordinator                  = "coordinator"
	overlord                     = "overlord"
	middleManager                = "middleManager"
	indexer                      = "indexer"
	historical                   = "historical"
	router                       = "router"
	defaultCommonConfigMountPath = "/druid/conf/druid/_common"
	finalizerName                = "deletepvc.finalizers.druid.apache.org"
)

//Initialize the slice to store node-specific unique identity strings for historical tiers
var historicalTierList []string
var logger = logf.Log.WithName("druid_operator_handler")

func deployDruidCluster(sdk client.Client, m *v1alpha1.Druid, emitEvents EventEmitter) error {
	if m.Spec.Ignored {
		return nil
	}

	if err := verifyDruidSpec(m); err != nil {
		e := fmt.Errorf("invalid DruidSpec[%s:%s] due to [%s]", m.Kind, m.Name, err.Error())
		emitEvents.EmitEventGeneric(m, "DruidOperatorInvalidSpec", "", e)
		return nil
	}

	allNodeSpecs, err := getAllNodeSpecsInDruidPrescribedOrder(m)
	if err != nil {
		e := fmt.Errorf("invalid DruidSpec[%s:%s] due to [%s]", m.Kind, m.Name, err.Error())
		emitEvents.EmitEventGeneric(m, "DruidOperatorInvalidSpec", "", e)
		return nil
	}

	statefulSetNames := make(map[string]bool)
	deploymentNames := make(map[string]bool)
	serviceNames := make(map[string]bool)
	configMapNames := make(map[string]bool)
	podDisruptionBudgetNames := make(map[string]bool)
	hpaNames := make(map[string]bool)
	ingressNames := make(map[string]bool)
	pvcNames := make(map[string]bool)

	ls := makeLabelsForDruid(m.Name)

	commonConfig, err := makeCommonConfigMap(m, ls)
	if err != nil {
		return err
	}
	commonConfigSHA, err := getObjectHash(commonConfig)
	if err != nil {
		return err
	}

	if _, err := sdkCreateOrUpdateAsNeeded(sdk,
		func() (object, error) { return makeCommonConfigMap(m, ls) },
		func() object { return makeConfigMapEmptyObj() },
		alwaysTrueIsEqualsFn, noopUpdaterFn, m, configMapNames, emitEvents); err != nil {
		return err
	}

	/*
		Default Behavior: Finalizer shall be always executed resulting in deletion of pvc post deletion of Druid CR
		When the object (druid CR) has for deletion time stamp set, execute the finalizer
		Finalizer shall execute the following flow :
		1. Get sts List and PVC List
		2. Range and Delete sts first and then delete pvc. PVC must be deleted after sts termination has been executed
			else pvc finalizer shall block deletion since a pod/sts is referencing it.
		3. Once delete is executed we block program and return.
	*/

	if m.Spec.DisablePVCDeletionFinalizer == false {
		md := m.GetDeletionTimestamp() != nil
		if md {
			return executeFinalizers(sdk, m, emitEvents)
		}
		/*
			If finalizer isn't present add it to object meta.
			In case cr is already deleted do not call this function
		*/
		cr := checkIfCRExists(sdk, m, emitEvents)
		if cr {
			if !ContainsString(m.ObjectMeta.Finalizers, finalizerName) {
				m.SetFinalizers(append(m.GetFinalizers(), finalizerName))
				_, err := writers.Update(context.Background(), sdk, m, m, emitEvents)
				if err != nil {
					return err
				}
			}
		}
	}

	for _, elem := range allNodeSpecs {
	    if !m.Spec.RollingDeploy {
	        break
	    }

	    key := elem.key
	    if elem.spec.NodeType == historical {
	        nodeSpecUniqueStr := makeNodeSpecificUniqueString(m, key)
	        historicalTierList = append(historicalTierList, nodeSpecUniqueStr)
	    }
	}


	for _, elem := range allNodeSpecs {
		key := elem.key
		nodeSpec := elem.spec

		//Name in k8s must pass regex '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'
		//So this unique string must follow same.
		nodeSpecUniqueStr := makeNodeSpecificUniqueString(m, key)

		lm := makeLabelsForNodeSpec(&nodeSpec, m, m.Name, nodeSpecUniqueStr)

		// create configmap first
		nodeConfig, err := makeConfigMapForNodeSpec(&nodeSpec, m, lm, nodeSpecUniqueStr)
		if err != nil {
			return err
		}

		nodeConfigSHA, err := getObjectHash(nodeConfig)
		if err != nil {
			return err
		}

		if _, err := sdkCreateOrUpdateAsNeeded(sdk,
			func() (object, error) { return nodeConfig, nil },
			func() object { return makeConfigMapEmptyObj() },
			alwaysTrueIsEqualsFn, noopUpdaterFn, m, configMapNames, emitEvents); err != nil {
			return err
		}

		//create services before creating statefulset
		firstServiceName := ""
		services := firstNonNilValue(nodeSpec.Services, m.Spec.Services).([]v1.Service)
		for _, svc := range services {
			if _, err := sdkCreateOrUpdateAsNeeded(sdk,
				func() (object, error) { return makeService(&svc, &nodeSpec, m, lm, nodeSpecUniqueStr) },
				func() object { return makeServiceEmptyObj() }, alwaysTrueIsEqualsFn,
				func(prev, curr object) { (curr.(*v1.Service)).Spec.ClusterIP = (prev.(*v1.Service)).Spec.ClusterIP },
				m, serviceNames, emitEvents); err != nil {
				return err
			}
			if firstServiceName == "" {
				firstServiceName = svc.ObjectMeta.Name
			}
		}

		nodeSpec.Ports = append(nodeSpec.Ports, v1.ContainerPort{ContainerPort: nodeSpec.DruidPort, Name: "druid-port"})

		if nodeSpec.Kind == "Deployment" {
			if deployCreateUpdateStatus, err := sdkCreateOrUpdateAsNeeded(sdk,
				func() (object, error) {
					return makeDeployment(&nodeSpec, m, lm, nodeSpecUniqueStr, fmt.Sprintf("%s-%s", commonConfigSHA, nodeConfigSHA), firstServiceName)
				},
				func() object { return makeDeploymentEmptyObj() },
				deploymentIsEquals, noopUpdaterFn, m, deploymentNames, emitEvents); err != nil {
				return err
			} else if m.Spec.RollingDeploy {

				if deployCreateUpdateStatus == resourceUpdated {
					return nil
				}

				// Ignore isObjFullyDeployed() for the first iteration ie cluster creation
				// will force cluster creation in parallel, post first iteration rolling updates
				// will be sequential.
				if m.Generation > 1 {
					// Check Deployment rolling update status, if in-progress then stop here
					done, err := isObjFullyDeployed(sdk, nodeSpec, nodeSpecUniqueStr, m, func() object { return makeDeploymentEmptyObj() }, emitEvents)
					if !done {
						return err
					}
				}
			}
		} else {

			//	scalePVCForSTS to be only called only if volumeExpansion is supported by the storage class.
			//  Ignore for the first iteration ie cluster creation, else get sts shall unnecessary log errors.

			if m.Generation > 1 && m.Spec.ScalePvcSts {
				if isVolumeExpansionEnabled(sdk, m, &nodeSpec, emitEvents) {
					err := scalePVCForSts(sdk, &nodeSpec, nodeSpecUniqueStr, m, emitEvents)
					if err != nil {
						return err
					}
				}
			}

			// In case of rollingDeploy, check if each historical tier has been successfully deployed before moving on
			if m.Spec.RollingDeploy && elem.spec.NodeType == historical {
			    for _, historicalTier := range historicalTierList {
			        done, err := isObjFullyDeployed(sdk, nodeSpec, nodeSpecUniqueStr, m, func() object { return makeStatefulSetEmptyObj() }, emitEvents)
                    if !done {
                        return err
                    }
			    }
			}

			// Create/Update StatefulSet
			if stsCreateUpdateStatus, err := sdkCreateOrUpdateAsNeeded(sdk,
				func() (object, error) {
					return makeStatefulSet(&nodeSpec, m, lm, nodeSpecUniqueStr, fmt.Sprintf("%s-%s", commonConfigSHA, nodeConfigSHA), firstServiceName)
				},
				func() object { return makeStatefulSetEmptyObj() },
				statefulSetIsEquals, noopUpdaterFn, m, statefulSetNames, emitEvents); err != nil {
				return err
			} else if m.Spec.RollingDeploy {

				if stsCreateUpdateStatus == resourceUpdated {
					// we just updated, give sts controller some time to update status of replicas after update
					return nil
				}

				// Default is set to true
				execCheckCrashStatus(sdk, &nodeSpec, m, emitEvents)

				// Ignore isObjFullyDeployed() for the first iteration ie cluster creation
				// will force cluster creation in parallel, post first iteration rolling updates
				// will be sequential.
				if m.Generation > 1 {
					//Check StatefulSet rolling update status, if in-progress then stop here
					done, err := isObjFullyDeployed(sdk, nodeSpec, nodeSpecUniqueStr, m, func() object { return makeStatefulSetEmptyObj() }, emitEvents)
					if !done {
						return err
					}
				}
			}

			// Default is set to true
			execCheckCrashStatus(sdk, &nodeSpec, m, emitEvents)
		}

		// Create Ingress Spec
		if nodeSpec.Ingress != nil {
			if _, err := sdkCreateOrUpdateAsNeeded(sdk,
				func() (object, error) {
					return makeIngress(&nodeSpec, m, ls, nodeSpecUniqueStr)
				},
				func() object { return makeIngressEmptyObj() },
				alwaysTrueIsEqualsFn, noopUpdaterFn, m, ingressNames, emitEvents); err != nil {
				return err
			}
		}

		// Create PodDisruptionBudget
		if nodeSpec.PodDisruptionBudgetSpec != nil {
			if _, err := sdkCreateOrUpdateAsNeeded(sdk,
				func() (object, error) { return makePodDisruptionBudget(&nodeSpec, m, lm, nodeSpecUniqueStr) },
				func() object { return makePodDisruptionBudgetEmptyObj() },
				alwaysTrueIsEqualsFn, noopUpdaterFn, m, podDisruptionBudgetNames, emitEvents); err != nil {
				return err
			}
		}

		// Create HPA Spec
		if nodeSpec.HPAutoScaler != nil {
			if _, err := sdkCreateOrUpdateAsNeeded(sdk,
				func() (object, error) {
					return makeHorizontalPodAutoscaler(&nodeSpec, m, ls, nodeSpecUniqueStr)
				},
				func() object { return makeHorizontalPodAutoscalerEmptyObj() },
				alwaysTrueIsEqualsFn, noopUpdaterFn, m, hpaNames, emitEvents); err != nil {
				return err
			}
		}

		if nodeSpec.PersistentVolumeClaim != nil {
			for _, pvc := range nodeSpec.PersistentVolumeClaim {
				if _, err := sdkCreateOrUpdateAsNeeded(sdk,
					func() (object, error) { return makePersistentVolumeClaim(&pvc, &nodeSpec, m, lm, nodeSpecUniqueStr) },
					func() object { return makePersistentVolumeClaimEmptyObj() }, alwaysTrueIsEqualsFn,
					noopUpdaterFn,
					m, pvcNames, emitEvents); err != nil {
					return err
				}
			}
		}
	}

	// Ignore on cluster creation
	if m.Generation > 1 && m.Spec.DeleteOrphanPvc {
		if err := deleteOrphanPVC(sdk, m, emitEvents); err != nil {
			return err
		}
	}

	//update status and delete unwanted resources
	updatedStatus := v1alpha1.DruidClusterStatus{}

	updatedStatus.StatefulSets = deleteUnusedResources(sdk, m, statefulSetNames, ls,
		func() objectList { return makeStatefulSetListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*appsv1.StatefulSetList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		}, emitEvents)
	sort.Strings(updatedStatus.StatefulSets)

	updatedStatus.Deployments = deleteUnusedResources(sdk, m, deploymentNames, ls,
		func() objectList { return makeDeloymentListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*appsv1.DeploymentList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		}, emitEvents)
	sort.Strings(updatedStatus.Deployments)

	updatedStatus.HPAutoScalers = deleteUnusedResources(sdk, m, hpaNames, ls,
		func() objectList { return makeHorizontalPodAutoscalerListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*autoscalev2beta2.HorizontalPodAutoscalerList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		}, emitEvents)
	sort.Strings(updatedStatus.HPAutoScalers)

	updatedStatus.Ingress = deleteUnusedResources(sdk, m, ingressNames, ls,
		func() objectList { return makeIngressListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*networkingv1.IngressList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		}, emitEvents)
	sort.Strings(updatedStatus.Ingress)

	updatedStatus.PodDisruptionBudgets = deleteUnusedResources(sdk, m, podDisruptionBudgetNames, ls,
		func() objectList { return makePodDisruptionBudgetListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*v1beta1.PodDisruptionBudgetList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		}, emitEvents)
	sort.Strings(updatedStatus.PodDisruptionBudgets)

	updatedStatus.Services = deleteUnusedResources(sdk, m, serviceNames, ls,
		func() objectList { return makeServiceListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*v1.ServiceList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		}, emitEvents)
	sort.Strings(updatedStatus.Services)

	updatedStatus.ConfigMaps = deleteUnusedResources(sdk, m, configMapNames, ls,
		func() objectList { return makeConfigMapListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*v1.ConfigMapList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		}, emitEvents)
	sort.Strings(updatedStatus.ConfigMaps)

	podList, _ := readers.List(context.TODO(), sdk, m, makeLabelsForDruid(m.Name), emitEvents, func() objectList { return makePodList() }, func(listObj runtime.Object) []object {
		items := listObj.(*v1.PodList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})

	updatedStatus.Pods = getPodNames(podList)
	sort.Strings(updatedStatus.Pods)

	// All druid nodes are in Ready state.
	// In case any druid node goes into a bad state, it shall be handled in above rollingDeploy block
	updatedStatus.DruidNodeStatus = *newDruidNodeTypeStatus(v1.ConditionTrue, v1alpha1.DruidClusterReady, "", nil)

	// In case of rolling Deploy not present OR any error not catched in the above block, check the pod ready
	// state and condition and patch the status with the CR
	for _, po := range podList {
		for _, c := range po.(*v1.Pod).Status.Conditions {
			if c.Type == v1.PodReady && c.Status == v1.ConditionFalse {
				updatedStatus.DruidNodeStatus = *newDruidNodeTypeStatus(v1.ConditionTrue, v1alpha1.DruidNodeErrorState, po.GetName(), errors.New(c.Reason))
			}
		}
	}

	err = druidClusterStatusPatcher(sdk, updatedStatus, m, emitEvents)
	if err != nil {
		return err
	}

	return nil
}

func deleteSTSAndPVC(sdk client.Client, drd *v1alpha1.Druid, stsList, pvcList []object, emitEvents EventEmitter) error {

	for _, sts := range stsList {
		err := writers.Delete(context.TODO(), sdk, drd, sts, emitEvents, &client.DeleteAllOfOptions{})
		if err != nil {
			return err
		}
	}

	for i := range pvcList {
		err := writers.Delete(context.TODO(), sdk, drd, pvcList[i], emitEvents, &client.DeleteAllOfOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func checkIfCRExists(sdk client.Client, m *v1alpha1.Druid, emitEvents EventEmitter) bool {
	_, err := readers.Get(context.TODO(), sdk, m.Name, m, func() object { return makeDruidEmptyObj() }, emitEvents)
	if err != nil {
		return false
	} else {
		return true
	}
}

func deleteOrphanPVC(sdk client.Client, drd *v1alpha1.Druid, emitEvents EventEmitter) error {

	podList, err := readers.List(context.TODO(), sdk, drd, makeLabelsForDruid(drd.Name), emitEvents, func() objectList { return makePodList() }, func(listObj runtime.Object) []object {
		items := listObj.(*v1.PodList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return err
	}

	pvcLabels := map[string]string{
		"druid_cr": drd.Name,
	}

	pvcList, err := readers.List(context.TODO(), sdk, drd, pvcLabels, emitEvents, func() objectList { return makePersistentVolumeClaimListEmptyObj() }, func(listObj runtime.Object) []object {
		items := listObj.(*v1.PersistentVolumeClaimList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return err
	}

	// Fix: https://github.com/druid-io/druid-operator/issues/149
	for _, pod := range podList {
		if pod.(*v1.Pod).Status.Phase != v1.PodRunning {
			return nil
		}
		for _, status := range pod.(*v1.Pod).Status.Conditions {
			if status.Status != v1.ConditionTrue {
				return nil
			}
		}
	}

	mountedPVC := make([]string, len(podList))
	for _, pod := range podList {
		if pod.(*v1.Pod).Spec.Volumes != nil {
			for _, vol := range pod.(*v1.Pod).Spec.Volumes {
				if vol.PersistentVolumeClaim != nil && pod.(*v1.Pod).Status.Phase != v1.PodPending {
					if !ContainsString(mountedPVC, vol.PersistentVolumeClaim.ClaimName) {
						mountedPVC = append(mountedPVC, vol.PersistentVolumeClaim.ClaimName)
					}
				}
			}
		}

	}

	if mountedPVC != nil {
		for i, pvc := range pvcList {

			if !ContainsString(mountedPVC, pvc.GetName()) {
				err := writers.Delete(context.TODO(), sdk, drd, pvcList[i], emitEvents, &client.DeleteAllOfOptions{})
				if err != nil {
					return err
				} else {
					msg := fmt.Sprintf("Deleted orphaned pvc [%s:%s] successfully", pvcList[i].GetName(), drd.Namespace)
					logger.Info(msg, "name", drd.Name, "namespace", drd.Namespace)
				}
			}
		}
	}
	return nil
}

func executeFinalizers(sdk client.Client, m *v1alpha1.Druid, emitEvents EventEmitter) error {

	if ContainsString(m.ObjectMeta.Finalizers, finalizerName) {
		pvcLabels := map[string]string{
			"druid_cr": m.Name,
		}

		pvcList, err := readers.List(context.TODO(), sdk, m, pvcLabels, emitEvents, func() objectList { return makePersistentVolumeClaimListEmptyObj() }, func(listObj runtime.Object) []object {
			items := listObj.(*v1.PersistentVolumeClaimList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		})
		if err != nil {
			return err
		}

		stsList, err := readers.List(context.TODO(), sdk, m, makeLabelsForDruid(m.Name), emitEvents, func() objectList { return makeStatefulSetListEmptyObj() }, func(listObj runtime.Object) []object {
			items := listObj.(*appsv1.StatefulSetList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		})
		if err != nil {
			return err
		}

		msg := fmt.Sprintf("Trigerring finalizer for CR [%s] in namespace [%s]", m.Name, m.Namespace)
		//		sendEvent(sdk, m, v1.EventTypeNormal, DruidFinalizer, msg)
		logger.Info(msg)
		if err := deleteSTSAndPVC(sdk, m, stsList, pvcList, emitEvents); err != nil {
			return err
		} else {
			msg := fmt.Sprintf("Finalizer success for CR [%s] in namespace [%s]", m.Name, m.Namespace)
			//			sendEvent(sdk, m, v1.EventTypeNormal, DruidFinalizerSuccess, msg)
			logger.Info(msg)
		}

		// remove our finalizer from the list and update it.
		m.ObjectMeta.Finalizers = RemoveString(m.ObjectMeta.Finalizers, finalizerName)

		_, err = writers.Update(context.TODO(), sdk, m, m, emitEvents)
		if err != nil {
			return err
		}

	}
	return nil

}

func execCheckCrashStatus(sdk client.Client, nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, event EventEmitter) {
	if m.Spec.ForceDeleteStsPodOnError == false {
		return
	} else {
		if nodeSpec.PodManagementPolicy == "OrderedReady" {
			checkCrashStatus(sdk, m, event)
		}
	}
}

func checkCrashStatus(sdk client.Client, drd *v1alpha1.Druid, emitEvents EventEmitter) error {

	podList, err := readers.List(context.TODO(), sdk, drd, makeLabelsForDruid(drd.Name), emitEvents, func() objectList { return makePodList() }, func(listObj runtime.Object) []object {
		items := listObj.(*v1.PodList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return err
	}

	for _, p := range podList {
		if p.(*v1.Pod).Status.ContainerStatuses[0].RestartCount > 1 {
			for _, condition := range p.(*v1.Pod).Status.Conditions {
				// condition.type Ready means the pod is able to service requests
				if condition.Type == v1.ContainersReady {
					// the below condition evalutes if a pod is in
					// 1. pending state 2. failed state 3. unknown state
					// OR condtion.status is false which evalutes if neither of these conditions are met
					// 1. ContainersReady 2. PodInitialized 3. PodReady 4. PodScheduled
					if p.(*v1.Pod).Status.Phase != v1.PodRunning || condition.Status == v1.ConditionFalse {
						err := writers.Delete(context.TODO(), sdk, drd, p, emitEvents, &client.DeleteOptions{})
						if err != nil {
							return err
						} else {
							msg := fmt.Sprintf("Deleted pod [%s] in namespace [%s], since it was in crashloopback state.", p.GetName(), p.GetNamespace())
							logger.Info(msg, "Object", stringifyForLogging(p, drd), "name", drd.Name, "namespace", drd.Namespace)
						}
					}
				}
			}
		}
	}

	return nil
}

func deleteUnusedResources(sdk client.Client, drd *v1alpha1.Druid,
	names map[string]bool, selectorLabels map[string]string, emptyListObjFn func() objectList, itemsExtractorFn func(obj runtime.Object) []object, emitEvents EventEmitter) []string {

	listOpts := []client.ListOption{
		client.InNamespace(drd.Namespace),
		client.MatchingLabels(selectorLabels),
	}

	survivorNames := make([]string, 0, len(names))

	listObj := emptyListObjFn()

	if err := sdk.List(context.TODO(), listObj, listOpts...); err != nil {
		e := fmt.Errorf("failed to list [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
	} else {
		for _, s := range itemsExtractorFn(listObj) {
			if names[s.GetName()] == false {
				err := writers.Delete(context.TODO(), sdk, drd, s, emitEvents, &client.DeleteOptions{})
				if err != nil {
					survivorNames = append(survivorNames, s.GetName())
				}
			} else {
				survivorNames = append(survivorNames, s.GetName())
			}
		}
	}

	return survivorNames
}

func alwaysTrueIsEqualsFn(prev, curr object) bool {
	return true
}

func noopUpdaterFn(prev, curr object) {
	// do nothing
}

func sdkCreateOrUpdateAsNeeded(
	sdk client.Client,
	objFn func() (object, error),
	emptyObjFn func() object,
	isEqualFn func(prev, curr object) bool,
	updaterFn func(prev, curr object),
	drd *v1alpha1.Druid,
	names map[string]bool,
	emitEvent EventEmitter) (DruidNodeStatus, error) {
	if obj, err := objFn(); err != nil {
		return "", err
	} else {
		names[obj.GetName()] = true

		addOwnerRefToObject(obj, asOwner(drd))
		addHashToObject(obj)

		prevObj := emptyObjFn()
		if err := sdk.Get(context.TODO(), *namespacedName(obj.GetName(), obj.GetNamespace()), prevObj); err != nil {
			if apierrors.IsNotFound(err) {
				// resource does not exist, create it.
				create, err := writers.Create(context.TODO(), sdk, drd, obj, emitEvent)
				if err != nil {
					return "", err
				} else {
					return create, nil
				}
			} else {
				e := fmt.Errorf("Failed to get [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
				logger.Error(e, e.Error(), "Prev object", stringifyForLogging(prevObj, drd), "name", drd.Name, "namespace", drd.Namespace)
				emitEvent.EmitEventGeneric(drd, string(druidOjectGetFail), "", err)
				return "", e
			}
		} else {
			// resource already exists, updated it if needed
			if obj.GetAnnotations()[druidOpResourceHash] != prevObj.GetAnnotations()[druidOpResourceHash] || !isEqualFn(prevObj, obj) {

				obj.SetResourceVersion(prevObj.GetResourceVersion())
				updaterFn(prevObj, obj)
				update, err := writers.Update(context.TODO(), sdk, drd, obj, emitEvent)
				if err != nil {
					return "", err
				} else {
					return update, err
				}
			} else {
				return "", nil
			}
		}
	}
}

func isObjFullyDeployed(sdk client.Client, nodeSpec v1alpha1.DruidNodeSpec, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object, emitEvent EventEmitter) (bool, error) {

	// Get Object
	obj, err := readers.Get(context.TODO(), sdk, nodeSpecUniqueStr, drd, emptyObjFn, emitEvent)
	if err != nil {
		return false, err
	}

	// In case obj is a statefulset or deployment, make sure the sts/deployment has successfully reconciled to desired state
	// TODO: @AdheipSingh once https://github.com/kubernetes/kubernetes/blob/master/pkg/apis/apps/types.go#L217 k8s conditions detect sts fail errors.
	if detectType(obj) == "*v1.StatefulSet" {
		if obj.(*appsv1.StatefulSet).Status.CurrentRevision != obj.(*appsv1.StatefulSet).Status.UpdateRevision {
			return false, nil
		} else if obj.(*appsv1.StatefulSet).Status.CurrentReplicas != obj.(*appsv1.StatefulSet).Status.ReadyReplicas {
			return false, nil
		} else {
			return obj.(*appsv1.StatefulSet).Status.CurrentRevision == obj.(*appsv1.StatefulSet).Status.UpdateRevision, nil
		}
	} else if detectType(obj) == "*v1.Deployment" {
		for _, condition := range obj.(*appsv1.Deployment).Status.Conditions {
			// This detects a failure condition, operator should send a rolling deployment failed event
			if condition.Type == appsv1.DeploymentReplicaFailure {
				return false, errors.New(condition.Reason)
			} else if condition.Type == appsv1.DeploymentProgressing && condition.Status != v1.ConditionTrue || obj.(*appsv1.Deployment).Status.ReadyReplicas != obj.(*appsv1.Deployment).Status.Replicas {
				return false, nil
			} else {
				return obj.(*appsv1.Deployment).Status.ReadyReplicas == obj.(*appsv1.Deployment).Status.Replicas, nil
			}
		}
	}
	return false, nil
}

// scalePVCForSts shall expand the sts volumeclaimtemplates size as well as N no of pvc supported by the sts.
// NOTE: To be called only if generation > 1
func scalePVCForSts(sdk client.Client, nodeSpec *v1alpha1.DruidNodeSpec, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emitEvent EventEmitter) error {

	getSTSList, err := readers.List(context.TODO(), sdk, drd, makeLabelsForDruid(drd.Name), emitEvent, func() objectList { return makeStatefulSetListEmptyObj() }, func(listObj runtime.Object) []object {
		items := listObj.(*appsv1.StatefulSetList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return nil
	}

	// Dont proceed unless all statefulsets are up and running.
	//  This can cause the go routine to panic

	for _, sts := range getSTSList {
		if sts.(*appsv1.StatefulSet).Status.Replicas != sts.(*appsv1.StatefulSet).Status.ReadyReplicas {
			return nil
		}
	}

	// return nil, in case return err the program halts since sts would not be able
	// we would like the operator to create sts.
	sts, err := readers.Get(context.TODO(), sdk, nodeSpecUniqueStr, drd, func() object { return makeStatefulSetEmptyObj() }, emitEvent)
	if err != nil {
		return nil
	}

	pvcLabels := map[string]string{
		"component": nodeSpec.NodeType,
	}

	pvcList, err := readers.List(context.TODO(), sdk, drd, pvcLabels, emitEvent, func() objectList { return makePersistentVolumeClaimListEmptyObj() }, func(listObj runtime.Object) []object {
		items := listObj.(*v1.PersistentVolumeClaimList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return nil
	}

	desVolumeClaimTemplateSize, currVolumeClaimTemplateSize, pvcSize := getVolumeClaimTemplateSizes(sts, nodeSpec, pvcList)

	// current number of PVC can't be less than desired number of pvc
	if len(pvcSize) < len(desVolumeClaimTemplateSize) {
		return nil
	}

	// iterate over array for matching each index in desVolumeClaimTemplateSize, currVolumeClaimTemplateSize and pvcSize
	for i := range desVolumeClaimTemplateSize {

		// Validate Request, shrinking of pvc not supported
		// desired size cant be less than current size
		// in that case re-create sts/pvc which is a user execute manual step

		desiredSize, _ := desVolumeClaimTemplateSize[i].AsInt64()
		currentSize, _ := currVolumeClaimTemplateSize[i].AsInt64()

		if desiredSize < currentSize {
			e := fmt.Errorf("Request for Shrinking of sts pvc size [sts:%s] in [namespace:%s] is not Supported", sts.(*appsv1.StatefulSet).Name, sts.(*appsv1.StatefulSet).Namespace)
			logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
			emitEvent.EmitEventGeneric(drd, "DruidOperatorPvcReSizeFail", "", err)
			return e
		}

		// In case size dont match and dessize > currsize, delete the sts using casacde=false / propagation policy set to orphan
		// The operator on next reconcile shall create the sts with latest changes
		if desiredSize != currentSize {
			msg := fmt.Sprintf("Detected Change in VolumeClaimTemplate Sizes for Statefuleset [%s] in Namespace [%s], desVolumeClaimTemplateSize: [%s], currVolumeClaimTemplateSize: [%s]\n, deleteing STS [%s] with casacde=false]", sts.(*appsv1.StatefulSet).Name, sts.(*appsv1.StatefulSet).Namespace, desVolumeClaimTemplateSize[i].String(), currVolumeClaimTemplateSize[i].String(), sts.(*appsv1.StatefulSet).Name)
			logger.Info(msg)
			emitEvent.EmitEventGeneric(drd, "DruidOperatorPvcReSizeDetected", msg, nil)

			if err := writers.Delete(context.TODO(), sdk, drd, sts, emitEvent, client.PropagationPolicy(metav1.DeletePropagationOrphan)); err != nil {
				return err
			} else {
				msg := fmt.Sprintf("[StatefuleSet:%s] successfully deleted with casacde=false", sts.(*appsv1.StatefulSet).Name)
				logger.Info(msg, "name", drd.Name, "namespace", drd.Namespace)
				emitEvent.EmitEventGeneric(drd, "DruidOperatorStsOrphaned", msg, nil)
			}

		}

		// In case size dont match, patch the pvc with the desiredsize from druid CR
		for p := range pvcSize {
			pSize, _ := pvcSize[p].AsInt64()
			if desiredSize != pSize {
				// use deepcopy
				patch := client.MergeFrom(pvcList[p].(*v1.PersistentVolumeClaim).DeepCopy())
				pvcList[p].(*v1.PersistentVolumeClaim).Spec.Resources.Requests[v1.ResourceStorage] = desVolumeClaimTemplateSize[i]
				if err := writers.Patch(context.TODO(), sdk, drd, pvcList[p].(*v1.PersistentVolumeClaim), false, patch, emitEvent); err != nil {
					return err
				} else {
					msg := fmt.Sprintf("[PVC:%s] successfully Patched with [Size:%s]", pvcList[p].(*v1.PersistentVolumeClaim).Name, desVolumeClaimTemplateSize[i].String())
					logger.Info(msg, "name", drd.Name, "namespace", drd.Namespace)
				}
			}
		}

	}

	return nil
}

// desVolumeClaimTemplateSize: the druid CR holds this value for a sts volumeclaimtemplate
// currVolumeClaimTemplateSize: the sts owned by druid CR holds this value in volumeclaimtemplate
// pvcSize: the pvc referenced by the sts holds this value
// type of vars is resource.Quantity. ref: https://godoc.org/k8s.io/apimachinery/pkg/api/resource
func getVolumeClaimTemplateSizes(sts object, nodeSpec *v1alpha1.DruidNodeSpec, pvc []object) (desVolumeClaimTemplateSize, currVolumeClaimTemplateSize, pvcSize []resource.Quantity) {

	for i := range nodeSpec.VolumeClaimTemplates {
		desVolumeClaimTemplateSize = append(desVolumeClaimTemplateSize, nodeSpec.VolumeClaimTemplates[i].Spec.Resources.Requests[v1.ResourceStorage])
	}

	for i := range sts.(*appsv1.StatefulSet).Spec.VolumeClaimTemplates {
		currVolumeClaimTemplateSize = append(currVolumeClaimTemplateSize, sts.(*appsv1.StatefulSet).Spec.VolumeClaimTemplates[i].Spec.Resources.Requests[v1.ResourceStorage])
	}

	for i := range pvc {
		pvcSize = append(pvcSize, pvc[i].(*v1.PersistentVolumeClaim).Spec.Resources.Requests[v1.ResourceStorage])
	}

	return desVolumeClaimTemplateSize, currVolumeClaimTemplateSize, pvcSize

}

func isVolumeExpansionEnabled(sdk client.Client, m *v1alpha1.Druid, nodeSpec *v1alpha1.DruidNodeSpec, emitEvent EventEmitter) bool {

	for _, nodeVCT := range nodeSpec.VolumeClaimTemplates {
		sc, err := readers.Get(context.TODO(), sdk, *nodeVCT.Spec.StorageClassName, m, func() object { return makeStorageClassEmptyObj() }, emitEvent)
		if err != nil {
			return false
		}

		if sc.(*storage.StorageClass).AllowVolumeExpansion != boolFalse() {
			return true
		}
	}
	return false
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

	data := map[string]string{
		"common.runtime.properties": prop,
	}

	if m.Spec.DimensionsMapPath != "" {
		data["metricDimensions.json"] = m.Spec.DimensionsMapPath
	}

	cfg, err := makeConfigMap(
		fmt.Sprintf("%s-druid-common-config", m.ObjectMeta.Name),
		m.Namespace,
		ls,
		data)
	return cfg, err
}

func makeConfigMapForNodeSpec(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, lm map[string]string, nodeSpecUniqueStr string) (*v1.ConfigMap, error) {

	data := map[string]string{
		"runtime.properties": fmt.Sprintf("druid.port=%d\n%s", nodeSpec.DruidPort, nodeSpec.RuntimeProperties),
		"jvm.config":         fmt.Sprintf("%s\n%s", firstNonEmptyStr(nodeSpec.JvmOptions, m.Spec.JvmOptions), nodeSpec.ExtraJvmOptions),
	}
	log4jconfig := firstNonEmptyStr(nodeSpec.Log4jConfig, m.Spec.Log4jConfig)
	if log4jconfig != "" {
		data["log4j2.xml"] = log4jconfig
	}

	return makeConfigMap(
		fmt.Sprintf("%s-config", nodeSpecUniqueStr),
		m.Namespace,
		lm,
		data)
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

func makeService(svc *v1.Service, nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr string) (*v1.Service, error) {
	svc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	}

	svc.ObjectMeta.Name = getServiceName(svc.ObjectMeta.Name, nodeSpecUniqueStr)

	svc.ObjectMeta.Namespace = m.Namespace

	if svc.ObjectMeta.Labels == nil {
		svc.ObjectMeta.Labels = ls
	} else {
		for k, v := range ls {
			svc.ObjectMeta.Labels[k] = v
		}
	}

	if svc.Spec.Selector == nil {
		svc.Spec.Selector = ls
	} else {
		for k, v := range ls {
			svc.Spec.Selector[k] = v
		}
	}

	if svc.Spec.Ports == nil {
		svc.Spec.Ports = []v1.ServicePort{
			{
				Name:       "service-port",
				Port:       nodeSpec.DruidPort,
				TargetPort: intstr.FromInt(int(nodeSpec.DruidPort)),
			},
		}
	}

	return svc, nil
}

func getServiceName(nameTemplate, nodeSpecUniqueStr string) string {
	if nameTemplate == "" {
		return nodeSpecUniqueStr
	} else {
		return fmt.Sprintf(nameTemplate, nodeSpecUniqueStr)
	}
}

func getPersistentVolumeClaim(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid) []v1.PersistentVolumeClaim {
	pvc := []v1.PersistentVolumeClaim{}

	for _, val := range m.Spec.VolumeClaimTemplates {
		pvc = append(pvc, val)
	}

	for _, val := range nodeSpec.VolumeClaimTemplates {
		pvc = append(pvc, val)
	}

	return pvc

}

func getVolumeMounts(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid) []v1.VolumeMount {
	volumeMount := []v1.VolumeMount{
		{
			MountPath: firstNonEmptyStr(m.Spec.CommonConfigMountPath, defaultCommonConfigMountPath),
			Name:      "common-config-volume",
			ReadOnly:  true,
		},
		{
			MountPath: firstNonEmptyStr(nodeSpec.NodeConfigMountPath, getNodeConfigMountPath(nodeSpec)),
			Name:      "nodetype-config-volume",
			ReadOnly:  true,
		},
	}

	volumeMount = append(volumeMount, m.Spec.VolumeMounts...)
	volumeMount = append(volumeMount, nodeSpec.VolumeMounts...)
	return volumeMount
}

func getNodeConfigMountPath(nodeSpec *v1alpha1.DruidNodeSpec) string {
	return fmt.Sprintf("/druid/conf/druid/%s", nodeSpec.NodeType)
}

func getTolerations(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid) []v1.Toleration {
	tolerations := []v1.Toleration{}

	for _, val := range m.Spec.Tolerations {
		tolerations = append(tolerations, val)
	}
	for _, val := range nodeSpec.Tolerations {
		tolerations = append(tolerations, val)
	}

	return tolerations
}

func getTopologySpreadConstraints(nodeSpec *v1alpha1.DruidNodeSpec) []v1.TopologySpreadConstraint {
	var topologySpreadConstraint []v1.TopologySpreadConstraint

	for _, val := range nodeSpec.TopologySpreadConstraints {
		topologySpreadConstraint = append(topologySpreadConstraint, val)
	}

	return topologySpreadConstraint
}

func getVolume(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, nodeSpecUniqueStr string) []v1.Volume {
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
	return volumesHolder
}

func getEnv(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, configMapSHA string) []v1.EnvVar {
	envHolder := firstNonNilValue(nodeSpec.Env, m.Spec.Env).([]v1.EnvVar)
	// enables to do the trick to force redeployment in case of configmap changes.
	envHolder = append(envHolder, v1.EnvVar{Name: "configMapSHA", Value: configMapSHA})

	return envHolder
}

func getAffinity(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid) *v1.Affinity {
	affinity := firstNonNilValue(m.Spec.Affinity, &v1.Affinity{}).(*v1.Affinity)
	affinity = firstNonNilValue(nodeSpec.Affinity, affinity).(*v1.Affinity)
	return affinity
}

func getLivenessProbe(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid) *v1.Probe {
	livenessProbe := updateDefaultPortInProbe(
		firstNonNilValue(nodeSpec.LivenessProbe, m.Spec.LivenessProbe).(*v1.Probe),
		nodeSpec.DruidPort)
	return livenessProbe
}

func getReadinessProbe(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid) *v1.Probe {
	readinessProbe := updateDefaultPortInProbe(
		firstNonNilValue(nodeSpec.ReadinessProbe, m.Spec.ReadinessProbe).(*v1.Probe),
		nodeSpec.DruidPort)
	return readinessProbe
}

func getStartUpProbe(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid) *v1.Probe {
	startUpProbe := updateDefaultPortInProbe(
		firstNonNilValue(nodeSpec.StartUpProbes, m.Spec.StartUpProbe).(*v1.Probe),
		nodeSpec.DruidPort)
	return startUpProbe
}

func getEnvFrom(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid) []v1.EnvFromSource {
	envFromHolder := firstNonNilValue(nodeSpec.EnvFrom, m.Spec.EnvFrom).([]v1.EnvFromSource)
	return envFromHolder
}

func getRollingUpdateStrategy(nodeSpec *v1alpha1.DruidNodeSpec) *appsv1.RollingUpdateDeployment {
	var nil *int32 = nil
	if nodeSpec.MaxSurge != nil || nodeSpec.MaxUnavailable != nil {
		return &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &intstr.IntOrString{
				IntVal: *nodeSpec.MaxUnavailable,
			},
			MaxSurge: &intstr.IntOrString{
				IntVal: *nodeSpec.MaxSurge,
			},
		}
	}
	return &appsv1.RollingUpdateDeployment{
		MaxUnavailable: &intstr.IntOrString{
			IntVal: int32(25),
		},
		MaxSurge: &intstr.IntOrString{
			IntVal: int32(25),
		},
	}

}

// makeStatefulSet shall create statefulset object.
func makeStatefulSet(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr, configMapSHA, serviceName string) (*appsv1.StatefulSet, error) {

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s", nodeSpecUniqueStr),
			Namespace: m.Namespace,
			Labels:    ls,
		},
		Spec: makeStatefulSetSpec(nodeSpec, m, ls, nodeSpecUniqueStr, configMapSHA, serviceName),
	}, nil
}

func statefulSetIsEquals(obj1, obj2 object) bool {

	// This used to match replica counts, but was reverted to fix https://github.com/druid-io/druid-operator/issues/160
	// because it is legitimate for HPA to change replica counts and operator shouldn't reset those.

	return true
}

// makeDeployment shall create deployment object.
func makeDeployment(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr, configMapSHA, serviceName string) (*appsv1.Deployment, error) {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s", nodeSpecUniqueStr),
			Namespace: m.Namespace,
			Labels:    ls,
		},
		Spec: makeDeploymentSpec(nodeSpec, m, ls, nodeSpecUniqueStr, configMapSHA, serviceName),
	}, nil
}

func deploymentIsEquals(obj1, obj2 object) bool {

	// This used to match replica counts, but was reverted to fix https://github.com/druid-io/druid-operator/issues/160
	// because it is legitimate for HPA to change replica counts and operator shouldn't reset those.

	return true
}

// makeStatefulSetSpec shall create statefulset spec for statefulsets.
func makeStatefulSetSpec(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecificUniqueString, configMapSHA, serviceName string) appsv1.StatefulSetSpec {

	updateStrategy := firstNonNilValue(m.Spec.UpdateStrategy, &appsv1.StatefulSetUpdateStrategy{}).(*appsv1.StatefulSetUpdateStrategy)
	updateStrategy = firstNonNilValue(nodeSpec.UpdateStrategy, updateStrategy).(*appsv1.StatefulSetUpdateStrategy)

	stsSpec := appsv1.StatefulSetSpec{
		ServiceName: serviceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: ls,
		},
		Replicas:             &nodeSpec.Replicas,
		PodManagementPolicy:  appsv1.PodManagementPolicyType(firstNonEmptyStr(firstNonEmptyStr(string(nodeSpec.PodManagementPolicy), string(m.Spec.PodManagementPolicy)), string(appsv1.ParallelPodManagement))),
		UpdateStrategy:       *updateStrategy,
		Template:             makePodTemplate(nodeSpec, m, ls, nodeSpecificUniqueString, configMapSHA),
		VolumeClaimTemplates: getPersistentVolumeClaim(nodeSpec, m),
	}

	return stsSpec

}

// makeDeploymentSpec shall create deployment Spec for deployments.
func makeDeploymentSpec(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecificUniqueString, configMapSHA, serviceName string) appsv1.DeploymentSpec {
	deploySpec := appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: ls,
		},
		Replicas: &nodeSpec.Replicas,
		Template: makePodTemplate(nodeSpec, m, ls, nodeSpecificUniqueString, configMapSHA),
		Strategy: appsv1.DeploymentStrategy{
			Type:          "RollingUpdate",
			RollingUpdate: getRollingUpdateStrategy(nodeSpec),
		},
	}

	return deploySpec
}

// makePodTemplate shall create podTemplate common to both deployment and statefulset.
func makePodTemplate(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr, configMapSHA string) v1.PodTemplateSpec {
	return v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      ls,
			Annotations: firstNonNilValue(nodeSpec.PodAnnotations, m.Spec.PodAnnotations).(map[string]string),
		},
		Spec: makePodSpec(nodeSpec, m, nodeSpecUniqueStr, configMapSHA),
	}
}

// makePodSpec shall create podSpec common to both deployment and statefulset.
func makePodSpec(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, nodeSpecUniqueStr, configMapSHA string) v1.PodSpec {

	var containers []v1.Container
	containers = append(containers,
		v1.Container{
			Image:           firstNonEmptyStr(nodeSpec.Image, m.Spec.Image),
			Name:            fmt.Sprintf("%s", nodeSpecUniqueStr),
			Command:         []string{firstNonEmptyStr(m.Spec.StartScript, "bin/run-druid.sh"), nodeSpec.NodeType},
			ImagePullPolicy: v1.PullPolicy(firstNonEmptyStr(string(nodeSpec.ImagePullPolicy), string(m.Spec.ImagePullPolicy))),
			Ports:           nodeSpec.Ports,
			Resources:       nodeSpec.Resources,
			Env:             getEnv(nodeSpec, m, configMapSHA),
			EnvFrom:         getEnvFrom(nodeSpec, m),
			VolumeMounts:    getVolumeMounts(nodeSpec, m),
			LivenessProbe:   getLivenessProbe(nodeSpec, m),
			ReadinessProbe:  getReadinessProbe(nodeSpec, m),
			StartupProbe:    getStartUpProbe(nodeSpec, m),
			Lifecycle:       nodeSpec.Lifecycle,
			SecurityContext: firstNonNilValue(nodeSpec.ContainerSecurityContext, m.Spec.ContainerSecurityContext).(*v1.SecurityContext),
		},
	)

	if m.Spec.AdditionalContainer != nil {

		for _, containerList := range m.Spec.AdditionalContainer {

			containers = append(containers,
				v1.Container{
					Image:           containerList.Image,
					Name:            containerList.ContainerName,
					Resources:       containerList.Resources,
					VolumeMounts:    containerList.VolumeMounts,
					Command:         containerList.Command,
					Args:            containerList.Args,
					ImagePullPolicy: containerList.ImagePullPolicy,
					SecurityContext: containerList.ContainerSecurityContext,
					Env:             containerList.Env,
					EnvFrom:         containerList.EnvFrom,
				},
			)
		}
	}

	spec := v1.PodSpec{
		NodeSelector:                  firstNonNilValue(m.Spec.NodeSelector, nodeSpec.NodeSelector).(map[string]string),
		TopologySpreadConstraints:     getTopologySpreadConstraints(nodeSpec),
		Tolerations:                   getTolerations(nodeSpec, m),
		Affinity:                      getAffinity(nodeSpec, m),
		ImagePullSecrets:              firstNonNilValue(nodeSpec.ImagePullSecrets, m.Spec.ImagePullSecrets).([]v1.LocalObjectReference),
		Containers:                    containers,
		TerminationGracePeriodSeconds: nodeSpec.TerminationGracePeriodSeconds,
		Volumes:                       getVolume(nodeSpec, m, nodeSpecUniqueStr),
		SecurityContext:               firstNonNilValue(nodeSpec.PodSecurityContext, m.Spec.PodSecurityContext).(*v1.PodSecurityContext),
		ServiceAccountName:            m.Spec.ServiceAccount,
	}
	return spec
}

func updateDefaultPortInProbe(probe *v1.Probe, defaultPort int32) *v1.Probe {
	if probe != nil && probe.HTTPGet != nil && probe.HTTPGet.Port.IntVal == 0 && probe.HTTPGet.Port.StrVal == "" {
		probe.HTTPGet.Port.IntVal = defaultPort
	}
	return probe
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

func makeHorizontalPodAutoscaler(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr string) (*autoscalev2beta2.HorizontalPodAutoscaler, error) {
	nodeHSpec := *nodeSpec.HPAutoScaler

	hpa := &autoscalev2beta2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "autoscaling/v2beta1",
			Kind:       "HorizontalPodAutoscaler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeSpecUniqueStr,
			Namespace: m.Namespace,
			Labels:    ls,
		},
		Spec: nodeHSpec,
	}

	return hpa, nil
}

func makeIngress(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr string) (*networkingv1.Ingress, error) {
	nodeIngressSpec := *nodeSpec.Ingress

	ingress := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        nodeSpecUniqueStr,
			Annotations: nodeSpec.IngressAnnotations,
			Namespace:   m.Namespace,
			Labels:      ls,
		},
		Spec: nodeIngressSpec,
	}

	return ingress, nil
}

func makePersistentVolumeClaim(pvc *v1.PersistentVolumeClaim, nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, ls map[string]string, nodeSpecUniqueStr string) (*v1.PersistentVolumeClaim, error) {

	pvc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
	}

	pvc.ObjectMeta.Namespace = m.Namespace

	if pvc.ObjectMeta.Labels == nil {
		pvc.ObjectMeta.Labels = ls
	} else {
		for k, v := range ls {
			pvc.ObjectMeta.Labels[k] = v
		}
	}

	if pvc.ObjectMeta.Name == "" {
		pvc.ObjectMeta.Name = nodeSpecUniqueStr
	} else {
		for _, p := range nodeSpec.PersistentVolumeClaim {
			pvc.ObjectMeta.Name = p.Name
			pvc.Spec = p.Spec
		}

	}

	return pvc, nil
}

// makeLabelsForDruid returns the labels for selecting the resources
// belonging to the given druid CR name.
func makeLabelsForDruid(name string) map[string]string {
	return map[string]string{"app": "druid", "druid_cr": name}
}

// makeLabelsForDruid returns the labels for selecting the resources
// belonging to the given druid CR name.
func makeLabelsForNodeSpec(nodeSpec *v1alpha1.DruidNodeSpec, m *v1alpha1.Druid, clusterName, nodeSpecUniqueStr string) map[string]string {
	var labels = map[string]string{}

	// if both labels are present at both cluster and node spec
	// labels should be merged.
	if nodeSpec.PodLabels != nil && m.Spec.PodLabels != nil {
		labels = nodeSpec.PodLabels
	}

	for k, v := range m.Spec.PodLabels {
		labels[k] = v
	}

	labels["app"] = "druid"
	labels["druid_cr"] = clusterName
	labels["nodeSpecUniqueStr"] = nodeSpecUniqueStr
	labels["component"] = nodeSpec.NodeType
	return labels
}

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}

// asOwner returns an OwnerReference set as the druid CR
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
func makePodList() *v1.PodList {
	return &v1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}
}

func makeDruidEmptyObj() *v1alpha1.Druid {
	return &v1alpha1.Druid{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Druid",
			APIVersion: "v1alpha1",
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

func makeDeloymentListEmptyObj() *appsv1.DeploymentList {
	return &appsv1.DeploymentList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
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

func makeHorizontalPodAutoscalerListEmptyObj() *autoscalev2beta2.HorizontalPodAutoscalerList {
	return &autoscalev2beta2.HorizontalPodAutoscalerList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "autoscaling/v2beta1",
			Kind:       "HorizontalPodAutoscaler",
		},
	}
}

func makeIngressListEmptyObj() *networkingv1.IngressList {
	return &networkingv1.IngressList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
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

func makePodEmptyObj() *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
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

func makeDeploymentEmptyObj() *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
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

func makeHorizontalPodAutoscalerEmptyObj() *autoscalev2beta2.HorizontalPodAutoscaler {
	return &autoscalev2beta2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "autoscaling/v2beta1",
			Kind:       "HorizontalPodAutoscaler",
		},
	}
}

func makePersistentVolumeClaimEmptyObj() *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
	}
}

func makePersistentVolumeClaimListEmptyObj() *v1.PersistentVolumeClaimList {
	return &v1.PersistentVolumeClaimList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
	}
}

func makeIngressEmptyObj() *networkingv1.Ingress {
	return &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
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

func makeStorageClassEmptyObj() *storage.StorageClass {
	return &storage.StorageClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
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
func getPodNames(pods []object) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.(*v1.Pod).Name)
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

	errorMsg := ""

	if drd.Spec.CommonRuntimeProperties == "" {
		errorMsg = fmt.Sprintf("%sCommonRuntimeProperties missing from Druid Cluster Spec\n", errorMsg)
	}

	if drd.Spec.CommonConfigMountPath == "" {
		errorMsg = fmt.Sprintf("%sCommonConfigMountPath missing from Druid Cluster Spec\n", errorMsg)
	}

	if drd.Spec.StartScript == "" {
		errorMsg = fmt.Sprintf("%sStartScript missing from Druid Cluster Spec\n", errorMsg)
	}

	for key, node := range drd.Spec.Nodes {
		if node.NodeType == "" {
			errorMsg = fmt.Sprintf("%sNode[%s] missing NodeType\n", errorMsg, key)
		}

		if drd.Spec.Image == "" && node.Image == "" {
			errorMsg = fmt.Sprintf("%sImage missing from Druid Cluster Spec\n", errorMsg)
		}

		if node.RuntimeProperties == "" {
			errorMsg = fmt.Sprintf("%sNode[%s] missing RuntimeProperties\n", errorMsg, key)
		}

		if node.NodeConfigMountPath == "" {
			errorMsg = fmt.Sprintf("%sNode[%s] missing NodeConfigMountPath\n", errorMsg, key)
		}

		if !keyValidationRegex.MatchString(key) {
			errorMsg = fmt.Sprintf("%sNode[%s] Key must match k8s resource name regex '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'", errorMsg, key)
		}
	}

	if errorMsg == "" {
		return nil
	} else {
		return fmt.Errorf(errorMsg)
	}
}

type keyAndNodeSpec struct {
	key  string
	spec v1alpha1.DruidNodeSpec
}

// Recommended prescribed order is described at http://druid.io/docs/latest/operations/rolling-updates.html
func getAllNodeSpecsInDruidPrescribedOrder(m *v1alpha1.Druid) ([]keyAndNodeSpec, error) {
	nodeSpecsByNodeType := map[string][]keyAndNodeSpec{
		historical:    make([]keyAndNodeSpec, 0, 1),
		overlord:      make([]keyAndNodeSpec, 0, 1),
		middleManager: make([]keyAndNodeSpec, 0, 1),
		indexer:       make([]keyAndNodeSpec, 0, 1),
		broker:        make([]keyAndNodeSpec, 0, 1),
		coordinator:   make([]keyAndNodeSpec, 0, 1),
		router:        make([]keyAndNodeSpec, 0, 1),
	}

	for key, nodeSpec := range m.Spec.Nodes {
		nodeSpecs := nodeSpecsByNodeType[nodeSpec.NodeType]
		if nodeSpecs == nil {
			return nil, fmt.Errorf("druidSpec[%s:%s] has invalid NodeType[%s]. Deployment aborted", m.Kind, m.Name, nodeSpec.NodeType)
		} else {
			nodeSpecsByNodeType[nodeSpec.NodeType] = append(nodeSpecs, keyAndNodeSpec{key, nodeSpec})
		}
	}

	allNodeSpecs := make([]keyAndNodeSpec, 0, len(m.Spec.Nodes))

	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[historical]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[overlord]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[middleManager]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[indexer]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[broker]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[coordinator]...)
	allNodeSpecs = append(allNodeSpecs, nodeSpecsByNodeType[router]...)

	return allNodeSpecs, nil
}

func namespacedName(name, namespace string) *types.NamespacedName {
	return &types.NamespacedName{Name: name, Namespace: namespace}
}
