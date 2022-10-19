# Features

* [Deny List in Operator](#Deny-List-in-Operator)
* [Reconcile Time in Operator](#Reconcile-Time-in-Operator)
* [Finalizer in Druid CR](#Finalizer-in-Druid-CR)
* [Deletetion of Orphan PVC's](#Deletetion-of-Orphan-PVC's)
* [Rolling Deploy](#Rolling-Deploy)
* [Force Delete of Sts Pods](#Force-Delete-of-Sts-Pods)
* [Scaling of Druid Nodes](#Scaling-of-Druid-Nodes)
* [Volume Expansion of Druid Nodes Running As StatefulSets](#Scaling-of-Druid-Nodes)
* [Add Additional Containers in Druid Nodes](#Add-Additional-Containers-in-Druid-Nodes)


## Deny List in Operator
- There may be use cases where we want the operator to watch all namespaces but restrict few namespaces, due to security, testing flexibility etc reasons.
- The druid operator supports such cases. In ```deploy/operator.yaml```, user can enable ```DENY_LIST``` env and pass the namespaces to be excluded.
- Each namespace to be seperated using a comma.

## Reconcile Time in Operator
- As per operator pattern, the druid operator reconciles every 10s ( default reconcile time ) to make sure the desired state ( druid CR ) in sync with current state.
- In case user wants to adjust the reconcile time, it can be adjusted by adding an ENV variable in ```deploy/operator.yaml```, user can enable ```RECONCILE_WAIT``` env and pass in the value suffixed with ```s``` string ( example: 30s). The default time is 10s.

## Finalizer in Druid CR
- Druid Operator supports provisioning of sts as well as deployments. When sts is created a pvc is created along. When druid CR is deleted the sts controller does not delete pvc's associated with sts.
- In case user does care about pvc data and wishes  to reclaim it, user can enable ```DisablePVCDeletionFinalizer: true``` in druid CR. 
- Default behavior shall trigger finalizers and pre-delete hooks that shall be executed which shall first clean up sts and then pvc referenced by sts.
- Default behavior is set to true ie after deletion of CR, any pvc's provisioned by sts shall be deleted.

## Deletetion of Orphan PVC's
- Assume ingestion is kicked off on druid, the sts MiddleManagers nodes are scaled to a certain number of replicas, and when the ingestion is completed. The middlemanagers are scaled down to avoid costs etc. 
- Sts on scale down, just terminates the pods it owns not the PVC. PVC are left orpahned and are of little or no use.
- In such cases druid-operator supports deletion of pvc orphaned by the sts. 
- To enable this feature users need to add a flag in the druid cluster spec ```deleteOrphanPvc: true```.

## Rolling Deploy
- Operator supports ```rollingDeploy```, in case specified to ```true``` at the clusterSpec, the operator does incremental updates in the order as mentioned [here](http://druid.io/docs/latest/operations/rolling-updates.html)
- In rollingDeploy each node is update one by one, and incase any of the node goes in pending/crashing state during update the operator halts the update and does not update the other nodes. This requires manual intervation.
- Default updates and cluster creation is in parallel. 
- Regardless of rolling deploy enabled, cluster creation always happens in parallel.

## Force Delete of Sts Pods
- During upgrade if sts is set to ordered ready, the sts controller will not recover from crashloopback state. The issues is referenced [here](https://github.com/kubernetes/kubernetes/issues/67250), and here's a reference [doc](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#forced-rollback)
- How operator solves this is using the ```forceDeleteStsPodOnError``` key, the operator will delete the sts pod if its in crashloopback state. Example Scenario: During upgrade, user rolls out a faulty configuration causing the historical pod going in crashing state, user rolls out a valid configuration, the new configuration will not be applied unless user manual delete pods, so solve this scenario operator shall delete the pod automatically without user intervention. 
- ```NOTE: User must be aware of this feature, there might be cases where crashloopback might be caused due probe failure, fault image etc, the operator shall keep on deleting on each re-concile loop. Default Behavior is True ```

## Scaling of Druid Nodes
- Operator supports ```HPA autosaling/v2beta2``` Spec in the nodeSpec for druid nodes. In case HPA deployed, HPA controller maintains the replica count/state for the particular statefulset referenced.  Refer to ```examples.md``` for HPA configuration. 
- ```NOTE: Prefered to scale only brokers using HPA.```
- In order to scale MM with HPA, its recommended not to use HPA. Refer to these discussions which have adderessed the issues in details.
1. https://github.com/apache/druid/issues/8801#issuecomment-664020630
2. https://github.com/apache/druid/issues/8801#issuecomment-664648399

## Volume Expansion of Druid Nodes Running As StatefulSets
```NOTE: This feature has been tested only on cloud environments and storage classes which have supported volume expansion. This feature uses cascade=orphan strategy to make sure only Stateful is deleted and recreated and pods are not deleted.```
- Druid Nodes specifically historicals run as statefulsets. Each statefulset replica has a pvc attached.
- NodeSpec in druid CR has key ```volumeClaimTemplates``` where users can define the pvc's storage class as well as size.
- In case a user wants to increase size in the node, the statefulsets cannot be directly updated.
- Druid Operator behind the scenes performs seamless update of the statefulset, plus patch the pvc's with desired size defined in the druid CR.
- Druid operator shall perform a cascade deletion of the sts, and shall patch the pvc. Cascade deletion has no affect to the pods running, queries are served and no downtime is experienced. 
- While enabling this feature, druid operator will check if volume expansion is supported in the storage class mentioned in the druid CR, only then will it perform expansion.
- Shrinkage of pvc's isnt supported, desiredSize cannot be less than currentSize as well as counts.
- To enable this feature ```scalePvcSts``` needs to be enabled to ```true```.
- By default, this feature is disabled.

## Add Additional Containers in Druid Nodes
- The Druid operator supports additional containers to run along with the druid services. This helps support co-located, co-managed helper processes for the primary druid application
- This can be used for init containers or sidecars or proxies etc. 
- To enable this features users just need to add a new container to the container list 
- This is scoped at cluster scope only, which means that additional container will be common to all the nodes
