# Druid Operator
[![Build Status](https://api.travis-ci.org/druid-io/druid-operator.svg?branch=master)](https://travis-ci.org/github/druid-io/druid-operator)
- Druid operator provisions and manages druid cluster on kubernetes. 
- It is built using the [operator-sdk](https://github.com/operator-framework/operator-sdk/tree/v0.11.0).
- Language used is GoLang 
- Refer to [Documentation](./docs/README.md) for getting started.

### Supported CR
- The operator supports ```Druid``` CR.
- ```Druid``` CR belongs to api Group ```druid.apache.org``` and version ```v1alpha1```


### Run the operator locally

Requirements:
  - Go 1.13
  - operator-sdk version v0.16.0.

```
$ docker build -t druid_image:tag . 
```

```
# set --watch-namespace with "" to watch all namespaces
$ operator-sdk run --local --watch-namespace=namespace
```
