## Example NodeSpec Supporting Mulitple Key/Values

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
      nodeConfigMountPath: /opt/druid/conf/druid/cluster/data/middleManager
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
             target:
               type: Utilization
               averageUtilization: 50

```

## Configure Ingress

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

## Configure Deployments 

```
  nodes:
    brokers:
      kind: Deployment
      nodeType: "broker"
      # Optionally specify for broker nodes
      # imagePullSecrets:
      # - name: tutu
      druid.port: 8088
      nodeConfigMountPath: "/opt/druid/conf/druid/cluster/query/broker"
      defaultReplicas: 1
```

## Configure Hot/Cold for Historicals

```
  hot:
    druid.port: 8083
    env:
    - name: DRUID_XMS
        value: 2g
    - name: DRUID_XMX
        value: 2g
    - name: DRUID_MAXDIRECTMEMORYSIZE
        value: 2g
    livenessProbe:
        failureThreshold: 3
        httpGet:
        path: /status/health
        port: 8083
        initialDelaySeconds: 1800
        periodSeconds: 5
    nodeConfigMountPath: /opt/druid/conf/druid/cluster/data/historical
    nodeType: historical
    podDisruptionBudgetSpec:
        maxUnavailable: 1
    readinessProbe:
        failureThreshold: 18
        httpGet:
        path: /druid/historical/v1/readiness
        port: 8083
        periodSeconds: 10
    replicas: 1
    resources:
        limits:
        cpu: 3
        memory: 6Gi
        requests:
        cpu: 1
        memory: 1Gi
    runtime.properties: 
        druid.plaintextPort=8083
        druid.service=druid/historical/hot
        ...
        ...
        ...

     cold:
        druid.port: 8083
        env:
        - name: DRUID_XMS
          value: 1500m
        - name: DRUID_XMX
          value: 1500m
        - name: DRUID_MAXDIRECTMEMORYSIZE
          value: 2g
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /status/health
            port: 8083
          initialDelaySeconds: 1800
          periodSeconds: 5
        nodeConfigMountPath: /opt/druid/conf/druid/cluster/data/historical
        nodeType: historical
        podDisruptionBudgetSpec:
          maxUnavailable: 1
        readinessProbe:
          failureThreshold: 18
          httpGet:
            path: /druid/historical/v1/readiness
            port: 8083
          periodSeconds: 10
        replicas: 1
        resources:
          limits:
            cpu: 4
            memory: 3.5Gi
          requests:
            cpu: 1
            memory: 1Gi
        runtime.properties: 
          druid.plaintextPort=8083
          druid.service=druid/historical/cold
          ...
          ...
          ...
```
