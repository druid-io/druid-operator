### Upgrade, Update and Scaling Druid cluster
- Operator supports ```rollingDeploy```, in case specified to ```true``` at the clusterSpec, the operator does incremental updates in the order as mentioned [here](http://druid.io/docs/latest/operations/rolling-updates.html)
- In rollingDeploy each node is update one by one, and incase any of the node goes in pending/crashing state during update the operator halts the update and does not update the other nodes. This requires manual intervation.
- Default updates and cluster creation is in parallel. 
- Operator supports ```HPA autosaling/v1beta1``` Spec in the nodeSpec for druid nodes. In case HPA deployed, HPA controller maintains the replica count/state for the particular statefulset referenced.  Refer to ```examples.md``` for HPA configuration. Prefered to scale only brokers using HPA.
