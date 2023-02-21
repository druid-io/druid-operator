# Kubernetes Operator for Apache Druid
![Build Status](https://github.com/druid-io/druid-operator/actions/workflows/docker-image.yml/badge.svg) ![Docker pull](https://img.shields.io/docker/pulls/druidio/druid-operator.svg) [![Latest Version](https://img.shields.io/github/tag/druid-io/druid-operator)](https://github.com/druid-io/druid-operator/releases)

# Deprecation Notice: 
- Refer to https://github.com/druid-io/druid-operator/issues/329.
- The project will be externally maintained in the following fork [druid-operator](https://github.com/datainfrahq/druid-operator)
- The new repo is fully backward compatible with previous releases. Feel free to open issues and collaborate. Thank You.

-------------------------------------------------------------------------------------------------------------------------------------------------------
- druid-operator provisions and manages [Apache Druid](https://druid.apache.org/) cluster on kubernetes.
- druid-operator is designed to provision and manage [Apache Druid](https://druid.apache.org/) in distributed mode only.
- It is built using the [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).
- Language used is GoLang.
- druid-operator is available on [operatorhub.io](https://operatorhub.io/operator/druid-operator)
- Refer to [Documentation](./docs/README.md) for getting started.
- Join Kubernetes slack and join [druid-operator](https://kubernetes.slack.com/archives/C04F4M6HT2L)

### Supported CR
- The operator supports CR of type ```Druid```.
- ```Druid``` CR belongs to api Group ```druid.apache.org``` and version ```v1alpha1```

### Notifications
- Users may experience HPA issues with druid-operator with release 0.0.5, as described in the [issue]( https://github.com/druid-io/druid-operator/issues/160). 
- The latest release 0.0.6 has fixes for the above issue.
- The operator has moved from HPA apiVersion autoscaling/v2beta1 to autoscaling/v2beta2 API users will need to update there HPA Specs according v2beta2 api in order to work with the latest druid-operator release.
- Users may experience pvc deletion [issue](https://github.com/druid-io/druid-operator/issues/186) in release 0.0.6, this issue has been fixed in patch release 0.0.6.1.
- druid-operator has moved Ingress apiVersion networking/v1beta1 to networking/v1. Users will need to update there Ingress Spec in the druid CR according networking/v1 syntax. In case users are using schema validated CRD, the CRD will also be needed to be updated.

### Note
Apache®, [Apache Druid, Druid®](https://druid.apache.org/) are either registered trademarks or trademarks of the Apache Software Foundation in the United States and/or other countries. This project, druid-operator, is not an Apache Software Foundation project.
