##  Druid CR Spec

- Druid CR has a ```clusterSpec``` which is common for all the druid nodes deployed and a ```nodeSpec``` which is specific to druid nodes.
- Some key values are ```Required```, ie they must be present in the spec to get the cluster deployed. Other's are optional.
- For full details on spec refer to ```pkg/apis/druid/v1alpha1/druid_types.go```
- The operator supports both deployments and statefulsets for druid Nodes. ```kind``` can be specified in the druid NodeSpec's to ```Deployment``` / ```StatefulSet```.
- ```NOTE: The default behavior shall provision all the nodes as statefulsets.```

- The following are cluster scoped and common to all the druid nodes. 

```
spec:
# Image for druid, Required Key
  image: apache/incubator-druid:0.16.0-incubating
  ....
  # Optionally specify image for all nodes. Can be specify on nodes also
  # imagePullSecrets:
  # - name: tutu
  ....
  # Entrypoint for druid cluster, Required Key
  startScript: /druid.sh
  ...
  # Labels populated by pods
  podLabels:
  ....
  # Pod Security Context 
  securityContext:
  ...
  # Service Spec created for all nodes
  services:
  ...
  # Mount Path to mount the common runtime,jvm and log4xml configs inside druid pods. Required Key
  commonConfigMountPath: "/opt/druid/conf/druid/cluster/_common"
  ...
  # JVM Options common for all druid nodes
  jvm.options: |-
  ...
  # log4j.config common for all druid nodes
  log4j.config: |-
  # common runtime properties for all druid nodes
  common.runtime.properties: |
 ```

 - The following are specific to a node.

 ```
  nodes:
    # String value, can be anything to define a node name.
    brokers:
      # nodeType can be broker,historical, middleManager, indexer, router, coordinator and overlord.
      # Required Key
      nodeType: "broker"
      # Optionally specify for broker nodes
      # imagePullSecrets:
      # - name: tutu
      # Port for the node
      druid.port: 8088
      # MountPath where are the all node properties get mounted as configMap
      nodeConfigMountPath: "/opt/druid/conf/druid/cluster/query/broker"
      # replica count, required must be greater than > 0.
      replicas: 1
      # Runtime Properties for the node
      # Required Key
      runtime.properties: |
```
