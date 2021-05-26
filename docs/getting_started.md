## Install the operator

- Register the `Druid` custom resource definition (CRD).

```
# Setup Service Account
druid-operator$ kubectl create -f deploy/service_account.yaml
# Setup RBAC
druid-operator$ kubectl create -f deploy/role.yaml
druid-operator$ kubectl create -f deploy/role_binding.yaml
# Setup the CRD
# Following CRD spec contains schema validation, you can find CRD spec without schema validation at
# deploy/crds/druid.apache.org_druids_crd.yaml
druid-operator$ kubectl create -f deploy/crds/druid.apache.org_druids.yaml

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

## Install the operator using Helm chart
- Install cluster scope operator into the `druid-operator` namespace:
```
# Create namespace
kubectl create namespace druid-operator

# Install Druid operator using Helm
helm -n druid-operator install cluster-druid-operator ./chart

# ... or generate manifest.yaml to install using other means:
helm -n druid-operator template cluster-druid-operator ./chart > manifest.yaml
```

- Install namespaced operator into the `druid-operator` namespace:
```
# Create namespace
kubectl create namespace druid-operator

# Install Druid operator using Helm
helm -n druid-operator install --set env.WATCH_NAMESPACE="mynamespace" namespaced-druid-operator ./chart
# you can use myvalues.yaml instead of --set
helm -n druid-operator install -f myvalues.yaml namespaced-druid-operator ./chart

# ... or generate manifest.yaml to install using other means:
helm -n druid-operator template --set env.WATCH_NAMESPACE="" namespaced-druid-operator ./chart > manifest.yaml
```

- Update settings, upgrade or rollback:
```
# To upgrade chart or apply changes in myvalues.yaml
helm -n druid-operator upgrade -f myvalues.yaml namespaced-druid-operator ./chart

# Rollback to previous revision
helm -n druid-operator rollback cluster-druid-operator
```

- Uninstall operator
```
# To avoid destroying existing clusters, helm will not uninstall its CRD. For 
# complete cleanup annotation needs to be removed first:
kubectl annotate crd druids.druid.apache.org helm.sh/resource-policy-

# This will uninstall operator
helm -n druid-operator uninstall cluster-druid-operator
```

## Deploy a sample Druid cluster

- An example spec to deploy a tiny druid cluster is included. For full details on spec please see `apis/druid/v1alpha1/druid_types.go`

```
# deploy single node zookeeper
druid-operator$ kubectl apply -f examples/tiny-cluster-zk.yaml

# deploy druid cluster spec
druid-operator$ kubectl apply -f examples/tiny-cluster.yaml
```

Note that above tiny-cluster only works on a single node kubernetes cluster(e.g. typical k8s cluster setup for dev using kind or minikube) as it uses local disk as "deep storage".

## Debugging Problems

 - For kubernetes version 1.11 make sure to disable ```type: object``` in the CRD root spec. 

```
# get druid-operator pod name
druid-operator$ kubectl get po | grep druid-operator

# check druid-operator pod logs
druid-operator$ kubectl logs <druid-operator pod name>

# check the druid spec
druid-operator$ kubectl describe druids tiny-cluster

#  check if druid cluster is deployed
druid-operator$ kubectl get svc | grep tiny
druid-operator$ kubectl get cm | grep tiny
druid-operator$ kubectl get sts | grep tiny
```
