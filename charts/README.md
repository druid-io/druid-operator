# Druid-operator Chart
- This chart installs Druid Operator, which can deploy druid cluster on kubernetes.

## Chart Details
- This chart has been tested using helm version 3. 
## Installing the Chart

To install the chart:

```
$ git clone https://github.com/druid-io/druid-operator.git
$ cd charts
$ helm install druid .
```

## Configuration

The following tables lists the configurable parameters of the druid Oparator chart and their default values.

| Parameter | Required | Description | Default value |
| --------- | -------- | ----------- | ------------- |
| replicaCount | yes | describes how many pods should handle the workload  | 1 |
| image.repository | yes | describes the image repository | druidio/druid-operator |
| image.tag | yes | describes the image tag | 0.0.1 |
| image.pullPolicy | yes | describes the image pullPolicy | IfNotPresent |
| rbac.enabled | yes | describes if the rbac is enabled in the cluster | true |
| rbac.apiversion | yes | describes rbac api version | v1beta1 |
| resources | no | resources can be specified to operator pods | {} |
| nodeSelector | no | nodeSelector can be specified to operator pods | {} |
| tolerations | no | tolerations can be specified to operator pods | {} |
| affinity | no | affinity can be specified to operator pods | {} |