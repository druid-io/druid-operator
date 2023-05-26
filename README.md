# This is a publicly accessible repo. Please don't commit sensitive content


**This is the official [druid-operator](https://github.com/druid-io/druid-operator)  project, now maintained by [Maintainers.md](./MAINTAINERS.md). 
[druid-operator](https://github.com/druid-io/druid-operator) is depreacted. Ref to [issue](https://github.com/druid-io/druid-operator/issues/329) and [PR](https://github.com/druid-io/druid-operator/pull/336). Feel free to open issues and PRs! Collaborators are welcome !**

# Kubernetes Operator for Apache Druid

![Build Status](https://github.com/datainfrahq/druid-operator/actions/workflows/docker-image.yml/badge.svg) ![Docker pull](https://img.shields.io/docker/pulls/datainfrahq/druid-operator.svg) [![Latest Version](https://img.shields.io/github/tag/datainfrahq/druid-operator)](https://github.com/datainfrahq/druid-operator/releases)  

- druid-operator provisions and manages [Apache Druid](https://druid.apache.org/) cluster on kubernetes.
- druid-operator is designed to provision and manage [Apache Druid](https://druid.apache.org/) in distributed mode only.
- It is built using the [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).
- Language used is GoLang.
- druid-operator is available on [operatorhub.io](https://operatorhub.io/operator/druid-operator)
- Refer to [Documentation](./docs/README.md) for getting started.
- Join Kubernetes slack and join [druid-operator](https://kubernetes.slack.com/archives/C04F4M6HT2L)

### Talks and Blogs on Druid Operator

- [Dok Community](https://www.youtube.com/live/X4A3lWJRGHk?feature=share)
- [Druid Summit](https://youtu.be/UqPrttXRBDg)
- [Druid Operator Blog](https://www.cloudnatively.com/apache-druid-on-kubernetes/)
- [Druid On K8s Without ZK](https://youtu.be/TRYOvkz5Wuw)

### Supported CR

- The operator supports CR of type ```Druid```.
- ```Druid``` CR belongs to api Group ```druid.apache.org``` and version ```v1alpha1```

### Druid Operator Architecture

![Druid Operator](docs/images/druid-operator.png?raw=true "Druid Operator")

### Notifications

- The project moved to <b>Kubebuilder v3</b> which requires a [manual change](docs/kubebuilder_v3_migration.md) in the operator. 
- Users may experience HPA issues with druid-operator with release 0.0.5, as described in the [issue](https://github.com/druid-io/druid-operator/issues/160).
- The latest release 0.0.6 has fixes for the above issue.
- The operator has moved from HPA apiVersion autoscaling/v2beta1 to autoscaling/v2 API users will need to update there HPA Specs according v2beta2 api in order to work with the latest druid-operator release.
- Users may experience pvc deletion [issue](https://github.com/druid-io/druid-operator/issues/186) in release 0.0.6, this issue has been fixed in patch release 0.0.6.1.
- druid-operator has moved Ingress apiVersion networking/v1beta1 to networking/v1. Users will need to update there Ingress Spec in the druid CR according networking/v1 syntax. In case users are using schema validated CRD, the CRD will also be needed to be updated.
- druid-operator has moved PodDisruptionBudget apiVersion policy/v1beta1 to policy/v1. Users will need to update there Kubernetes versions to 1.21+ to use druid-operator tag 0.0.9+.
- The latest release for druid-operator is v1.0.0, this release is compatible with k8s version 1.25. HPA API is kept to version v2beta2.

### Kubernetes version compatibility

| druid-operator | 0.0.9 | v1.0.0 |
| :------------- | :-------------: | :-----: |
| kubernetes <= 1.20 | :x:| :x: |
| kubernetes == 1.21 | :white_check_mark:| :x: |
| kubernetes >= 1.22 and < 1.25 | :white_check_mark: | :white_check_mark: |
| kubernetes > 1.25 | :x: | :white_check_mark: |

### Contributors

<a href="https://github.com/datainfrahq/druid-operator/graphs/contributors"><img src="https://contrib.rocks/image?repo=datainfrahq/druid-operator" /></a>

### Note
Apache®, [Apache Druid, Druid®](https://druid.apache.org/) are either registered trademarks or trademarks of the Apache Software Foundation in the United States and/or other countries. This project, druid-operator, is not an Apache Software Foundation project.
