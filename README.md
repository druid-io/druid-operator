# Druid Operator
[![Build Status](https://api.travis-ci.org/druid-io/druid-operator.svg?branch=master)](https://travis-ci.org/github/druid-io/druid-operator)
- Druid operator provisions and manages druid cluster on kubernetes. 
- It is built using the [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).
- Language used is GoLang 
- Refer to [Documentation](./docs/README.md) for getting started.

### Supported CR
- The operator supports ```Druid``` CR.
- ```Druid``` CR belongs to api Group ```druid.apache.org``` and version ```v1alpha1```
