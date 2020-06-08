# Druid Operator
[![Build Status](https://api.travis-ci.org/druid-io/druid-operator.svg?branch=master)](https://travis-ci.org/github/druid-io/druid-operator)
- Druid operator provisions and manages druid cluster on kubernetes. 
- It is built using the [operator-sdk](https://github.com/operator-framework/operator-sdk/tree/v0.11.0) . 
- Currently the operator deploys all the pods as statefulsets.
## Table of Contents
 * [Usage](#usage)    
    * [Install the Operator](#install-the-operator)
    * [Deploy a sample Druid Cluster](#deploy-a-sample-druid-cluster)
    * [Druid CR Spec](#druid-cr-spec)
    * [Example NodeSpec Supporting Multiple Options](#example-nodespec-supporting-multiple-options)
    * [Configure Ingress Example](#configure-ingress-example)
    * [Upgrade Update and Scaling Druid Cluster](#upgrade-update-and-scaling-druid-cluster)
    * [Run the Operator locally](#run-the-operator-locally)
## Usage

### Install the operator
- Register the `Druid` custom resource definition (CRD).
```
# Setup Service Account
druid-operator$ kubectl create -f deploy/service_account.yaml
# Setup RBAC
druid-operator$ kubectl create -f deploy/role.yaml
druid-operator$ kubectl create -f deploy/role_binding.yaml
# Setup the CRD
druid-operator$ kubectl create -f deploy/crds/druid.apache.org_druids_crd.yaml

# Update the operator manifest to use the druid-operator image name (if you are performing these steps on OSX, see note below)
druid-operator$ sed -i 's|REPLACE_IMAGE|<druid-operator-image>|g' deploy/operator.yaml
# On OSX use:
druid-operator$ sed -i "" 's|REPLACE_IMAGE|<druid-operator-image>|g' deploy/operator.yaml

# Deploy the druid-operator
druid-operator$ kubectl create -f deploy/operator.yaml

# Check the deployed druid-operator
druid-operator$ kubectl describe deployment druid-operator
```
- Operator can be deployed with namespaced scope or clutser scope. By default the operator is namespaced scope.
- For the operator to be cluster scope, the following params inside the ```operator.yaml``` need to be changed.
```
- name: WATCH_NAMESPACE
  value: ""
```
- Use ```ClusterRole``` and ```CluterRoleBinding``` instead of ```role```and ```roleBinding```.

### Deploy a sample Druid cluster
- An example spec to deploy a tiny druid cluster is included. For full details on spec please see `pkg/api/druid/v1alpha1/druid_types.go`

```
# deploy single node zookeeper
druid-operator$ kubectl apply -f examples/tiny-cluster-zk.yaml

# deploy druid cluster spec
druid-operator$ kubectl apply -f examples/tiny-cluster.yaml
```
###  Druid CR Spec
- Druid CR has a ```clusterSpec``` which is common for all the druid nodes deployed and a ```nodeSpec``` which is specific to druid nodes.
- Some key values are ```Required```, ie they must be present in the spec to get the cluster deployed. Other's are optional.
- For full details on spec refer to ```pkg/api/druid/v1alpha1/druid_types.go```
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
### Example NodeSpec Supporting Mulitple Options
```
    middlemanagers:
      podAnnotations:
        type: middlemanager
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              -
                matchExpressions:
                  -
                    key: node-type
                    operator: In
                    values:
                      - druid-data
      tolerations:
       -
         effect: NoSchedule
         key: node-role.kubernetes.io/master
         operator: Exists
      druid.port: 8091
      extra.jvm.options: |-
          -Xmx4G
          -Xms4G
      nodeType: middleManager
      nodeConfigMountPath: /opt/druid/conf/druid/cluster/data/middlemanager
      log4j.config: |-
          <Configuration status="WARN">
            <Appenders>
                <Console name="logline" target="SYSTEM_OUT">
                <PatternLayout pattern="%d{ISO8601} %p [%t] %c - %m%n"/>
              </Console>
              <Console name="msgonly" target="SYSTEM_OUT">
                <PatternLayout pattern="%m%n"/>
              </Console>
            </Appenders>
            <Loggers>
              <Root level="info">
                <AppenderRef ref="logline"/>
              </Root>
              <Logger name="org.apache.druid.java.util.emitter.core.LoggingEmitter" additivity="false" level="info">
                <AppenderRef ref="msgonly"/>
              </Logger>
            </Loggers>
          </Configuration>
      podDisruptionBudgetSpec:
        maxUnavailable: 1
      ports:
        -
          containerPort: 8100
          name: peon-0
      replicas: 1
      resources:
        limits:
          cpu: "2"
          memory: 5Gi
        requests:
          cpu: "2"
          memory: 5Gi
      livenessProbe:
          initialDelaySeconds: 30
          httpGet:
            path: /status/health
            port: 8091
      readinessProbe:
          initialDelaySeconds: 30
          httpGet:
            path: /status/health
            port: 8091
      runtime.properties: |-
          druid.service=druid/middleManager
          druid.worker.capacity=4
          druid.indexer.task.baseTaskDir=/druid/data/baseTaskDir
          druid.server.http.numThreads=10
          druid.indexer.fork.property.druid.processing.buffer.sizeBytes=1
          druid.indexer.fork.property.druid.processing.numMergeBuffers=1
          druid.indexer.fork.property.druid.processing.numThreads=1
          # Processing threads and buffers on Peons
          druid.indexer.fork.property.druid.processing.numMergeBuffers=2
          druid.indexer.fork.property.druid.processing.buffer.sizeBytes=100000000
          druid.indexer.fork.property.druid.processing.numThreads=1
      services:
        -
          spec:
            clusterIP: None
            ports:
              -
                name: tcp-service-port
                port: 8091
                targetPort: 8091
            type: ClusterIP
      volumeClaimTemplates:
        -
          metadata:
            name: data-volume
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 30Gi
            storageClassName: zone-a-storage
      volumeMounts:
        -
          mountPath: /druid/data
          name: data-volume
      containerSecurityContext:
        fsGroup: 1001
        capabilites:
          drop: ["ALL"]
      securityContext:
        runAsUser: 1001
      terminationGracePeriods: 100
      hpAutoscaler:
        maxReplicas: 10
        minReplicas: 1
        scaleTargetRef:
           apiVersion: apps/v1
           kind: StatefulSet
           name: druid-cluster-middlemanagers
        metrics:
         - type: Resource
           resource:
            name: cpu
            targetAverageUtilization: 60
         - type: Resource
           resource:
             name: memory
             targetAverageUtilization: 60
```
### Configure Ingress Example
```
 brokers:
      nodeType: "broker"
      druid.port: 8080
      ingressAnnotations:
          "nginx.ingress.kubernetes.io/rewrite-target": "/"
      ingress:
        tls:
         - hosts:
            - sslexample.foo.com
           secretName: testsecret-tls
        rules:
         - host: sslexample.foo.com
           http:
             paths:
             - path: /
               backend:
                 serviceName: service1
                 servicePort: 80    
```

### Upgrade Update and Scaling Druid cluster
- Operator supports ```rollingDeploy```, in case specified to ```true``` at the clusterSpec, the operator does incremental updates in the order as mentioned [here](http://druid.io/docs/latest/operations/rolling-updates.html)
- In rollingDeploy each node is update one by one, and incase any of the node goes in pending/crashing state during update the operator halts the update and does not update the other nodes. This requires manual intervation.
- Default updates and cluster creation is in parallel. 
- Operator support ```HPA autosaling/v1beta1``` Spec in the nodeSpec for druid nodes. In case HPA deployed, HPA controller maintains the replica count/state for the particular statefulset referenced.  Refer to the above spec for HPA configuration.

### Run the operator locally

Requirements:
  - Go 1.11+

```
$ docker build -t druid_image:tag . 
```

```
# set --watch-namespace with "" to watch all namespaces
$ operator-sdk run --local --watch-namespace=namespace
```
