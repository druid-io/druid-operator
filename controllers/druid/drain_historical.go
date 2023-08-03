package druid

import (
	"context"
	"fmt"
	"os"

	//TODO cleanup imports

	"github.com/datainfrahq/druid-operator/apis/druid/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger_drain_historical = logf.Log.WithName("drain_historical")

type CoordinatorConfig struct {
	decommissioningNodes []string
}

func checkDrainStatus(d DruidClient, start int32, end int32, podNames []string) (bool, error) {
	historicalUsageStats, err := d.GetHistoricalUsage()
	if err != nil {
		return false, err
	}
	for it := start; it <= end; it++ {
		if historicalUsageStats[podNames[it]] != 0 {
			return false, nil
		}
	}
	return true, nil
}

func patchUpdateStrategy(sdk client.Client, m *v1alpha1.Druid, nodeSpec *v1alpha1.DruidNodeSpec, strategy string, emitEvent EventEmitter) error {
	updatedObj := nodeSpec.DeepCopy()

	newStrategy := appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.OnDeleteStatefulSetStrategyType,
	}

	updatedObj.UpdateStrategy = &newStrategy

	if err := writers.Patch(context.TODO(), sdk, m, updatedObj, false, client.MergeFrom(nodeSpec), emitEvent); err != nil {
		return err
	} else {
		msg := fmt.Sprintf("Successfully Patched %s with %s", nodeSpec.UpdateStrategy, onDelete)
		logger_drain_historical.Info(msg, "name", m.Name, "namespace", m.Namespace)
		return nil
	}
}

func scaleStatefulSet(sdk client.Client, m *v1alpha1.Druid, nodeSpec *v1alpha1.DruidNodeSpec, strategy string, emitEvent EventEmitter, scaleUpCount int32) error {
	updatedObj := nodeSpec.DeepCopy()
	newReplica := int32(m.Status.Historical.Replica + scaleUpCount)
	updatedObj.Replicas = newReplica
	if err := writers.Patch(context.TODO(), sdk, m, updatedObj, false, client.MergeFrom(nodeSpec), emitEvent); err != nil {
		return err
	} else {
		msg := fmt.Sprintf("Successfully Patched replicas with %d", newReplica)
		logger_drain_historical.Info(msg, "name", m.Name, "namespace", m.Namespace)
		return nil
	}
}

func deployHistorical(sdk client.Client, m *v1alpha1.Druid, nodeSpec *v1alpha1.DruidNodeSpec, nodeSpecUniqueStr string, emitEvent EventEmitter, batchSize int32, baseUrl string) error {
	// patch the updateStrategy with onDelete
	err := patchUpdateStrategy(sdk, m, nodeSpec, onDelete, emitEvent)
	if err != nil {
		return err
	}
	// sts first scale up the pods by a batchsize and then drain the older pods
	if m.Status.Historical.CurrentBatch == 0 {
		m.Status.Historical = v1alpha1.HistoricalStatus{}
		m.Status.Historical.Replica = nodeSpec.Replicas
		m.Status.Historical.DecommissionedPods = []string{}

		err := scaleStatefulSet(sdk, m, nodeSpec, onDelete, emitEvent, batchSize)
		if err != nil {
			return err
		}

		obj, err := readers.Get(context.TODO(), sdk, nodeSpecUniqueStr, m, func() object { return &appsv1.StatefulSet{} }, emitEvent)
		if err != nil {
			return err
		}

		if obj.(*appsv1.StatefulSet).Status.ReadyReplicas != obj.(*appsv1.StatefulSet).Status.CurrentReplicas {
			msg := fmt.Sprintf("StatefulSet[%s] roll out is in progress CurrentReplicas[%d] != ReadyReplicas[%d]", nodeSpecUniqueStr, obj.(*appsv1.StatefulSet).Status.CurrentReplicas, obj.(*appsv1.StatefulSet).Status.ReadyReplicas)
			logger_drain_historical.Info(msg, "name", m.Name, "namespace", m.Namespace)
			return nil
		}

		m.Status.Historical.Replica = obj.(*appsv1.StatefulSet).Status.CurrentReplicas
		m.Status.Historical.CurrentBatch = 0
	}

	originalReplicaCount := m.Status.Historical.Replica - batchSize

	podList, err := readers.List(context.TODO(), sdk, m, makeLabelsForNodeSpec(nodeSpec, m, m.Name, nodeSpecUniqueStr), emitEvent, func() objectList { return &v1.PodList{} }, func(listObj runtime.Object) []object {
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
		"druid_cr":          m.Name,
		"nodeSpecUniqueStr": nodeSpecUniqueStr,
		"component":         nodeSpec.NodeType,
	}

	pvcList, err := readers.List(context.TODO(), sdk, m, pvcLabels, emitEvent, func() objectList { return &v1.PersistentVolumeClaimList{} }, func(listObj runtime.Object) []object {
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

	podNames := getPodNames(podList)
	//start draining the historical pod in batches and delete them
	batchCount := originalReplicaCount/nodeSpec.DeploymentConfig.BatchSize + 1

	//TODO add auth
	d := InitClient(baseUrl, userName, os.Getenv("DRUID_PASSWORD"))
	CoordinatorConfig, err := d.GetCoordinatorConfig()

	if err != nil {
		return err
	}
	startPod := m.Status.Historical.CurrentBatch * nodeSpec.DeploymentConfig.BatchSize
	endPod := startPod + nodeSpec.DeploymentConfig.BatchSize - 1
	if m.Status.Historical.CurrentBatch == batchCount {
		endPod = originalReplicaCount - 1
	}
	if m.Status.Historical.DecommissionedPods == nil {
		for podId := startPod; podId <= endPod; podId++ {
			m.Status.Historical.DecommissionedPods = append(m.Status.Historical.DecommissionedPods, podNames[podId])
		}
		CoordinatorConfig["decommissioningNodes"] = []string(m.Status.Historical.DecommissionedPods) //TODO fix this
		err := d.UpdateCoordinatorConfig(CoordinatorConfig)
		if err != nil {
			return err
		}
	}
	//wait for draining to complete so we can start with the next batch
	drainStatus, err := checkDrainStatus(d, startPod, endPod, podNames)
	if err != nil {
		return err
	}
	if !drainStatus {
		logger_drain_historical.Info("Waiting for pods to drain", "name", m.Name, "namespace", m.Namespace)
		return nil
	}
	//delete corresponding nodegrabbers
	for m.Status.Historical.DecommissionedPods != nil {
		//get the PVC for the pod to be disabled and delete it once the pod is deleted
		for _, pod := range podList {
			if pod.(*v1.Pod).Name == m.Status.Historical.DecommissionedPods[0] {
				err := writers.Delete(context.TODO(), sdk, m, pod, emitEvent, &client.DeleteOptions{})
				if err != nil {
					return err
				} else {
					// msg := fmt.Sprintf("Deleted pod [%s] in namespace [%s], since it was drained completely", m.Status.Historical.DecommissionedPods[0].GetName(), m.Status.Historical.DecommissionedPods[0].GetNamespace())
					// logger_drain_historical.Info(msg, "Object", stringifyForLogging(m.Status.Historical.DecommissionedPods[0], m), "name", m.Name, "namespace", m.Namespace)
					m.Status.Historical.DecommissionedPods = m.Status.Historical.DecommissionedPods[1:]
				}
			}
		}
	}
	for it := startPod; it <= endPod; it++ {
		err := writers.Delete(context.TODO(), sdk, m, pvcList[it], emitEvent, &client.DeleteAllOfOptions{})
		if err != nil {
			return err
		}
	}
	m.Status.Historical.CurrentBatch++
	return nil
}
