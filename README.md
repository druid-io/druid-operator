# Druid Operator
![Build Status](https://github.com/druid-io/druid-operator/actions/workflows/docker-image.yml/badge.svg)
- Druid operator provisions and manages druid cluster on kubernetes. 
- It is built using the [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).
- Language used is GoLang 
- Refer to [Documentation](./docs/README.md) for getting started.

### Supported CR
- The operator supports ```Druid``` CR.
- ```Druid``` CR belongs to api Group ```druid.apache.org``` and version ```v1alpha1```

### Notifications
- Users may experience HPA issues with druid operator with release 0.0.5, as described in the issue https://github.com/druid-io/druid-operator/issues/160. 
- The latest release 0.0.6 has fixes for the above issue.
- The operator has moved from HPA apiVersion autoscaling/v2beta1 to autoscaling/v2beta2 API users will need to update there HPA Specs according v2beta2 api in order to work with the latest druid operator release.
