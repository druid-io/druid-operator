# Druid on minikube
This doc has step-by-step instructions on how to deploy a local druid cluster on [minikube](https://github.com/kubernetes/minikube).
I chose minikube instead of a local docker install because it works for both Linux and MacOS. 


## 1. Setup minikube

a. Install minikube ([installation instructions](https://minikube.sigs.k8s.io/docs/start/))

b. Start a minikube cluster. I chose to allocate 4 CPUs, 12 GB of RAM and 60G of disk. 
```shell script
minikube start  --cpus 4 --memory 12192 -v 9 --disk-size 60G
```

c. Enable the ingress controller and ingress-dns addon.
```shell script
minikube addons enable ingress
minikube addons enable ingress-dns
```

Please make sure that you follow the steps [here](https://github.com/kubernetes/minikube/tree/master/deploy/addons/ingress-dns) to add minikube as the dns resolver for `*.test` domain. 

Hint: On MacOS you will need to kill the mDNSResponder for the dns changes to start working and also reload Mac OS mDNS resolver.
```shell script
sudo killall -HUP mDNSResponder
sudo launchctl unload -w /System/Library/LaunchDaemons/com.apple.mDNSResponder.plist
sudo launchctl load -w /System/Library/LaunchDaemons/com.apple.mDNSResponder.plist
```

## 2. Install minio (storage server with S3 compatible api) for deep storage

a. Install the chart.
```shell script
helm repo add minio https://helm.min.io/

kubectl create ns minio

helm upgrade \
    --install minio minio/minio \
    --namespace minio \
    --set resources.requests.memory=2Gi \
    --set persistence.size=500Gi \
    --set accessKey=AKIAIOSFODNN7EXAMPLE \
    --set secretKey="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" \
    --set ingress.enabled=true \
    --set ingress.hosts={minio.tiny.test} \
    --debug
```

b. Verify that the pods are healthy and check logs.
```shell script
kubectl get pods -n minio
kubectl -n minio logs -l app=minio
```
c. Access the minio web console at http://minio.tiny.test (use the apikey and access key from the Helm command in step a )
If this doesnot work make sure the ingress-dns and ingress controller are setup correctly (step c. in the section on minikube)

d. Create a bucket named `druidio` from the minio web console.

## 3. Install Postgres for metadata storage

a. Create a namespace. We will install the druid cluster in this namespace. If you change the namespace here, take care to update the namespace all of the other components below.
```shell script
kubectl create ns tiny-cluster
```
b. Install the postgres chart.
```shell script
helm repo add bitnami https://charts.bitnami.com/bitnami

helm upgrade \
  --install druid-metadata bitnami/postgresql \
  --namespace tiny-cluster \
  --set postgresqlUsername=druidpguser \
  --set postgresqlPassword=druidpgpassword \
  --set postgresqlDatabase=druiddb \
  --debug
```

c. Make sure the postgres pods and up and running.
```shell script
kubectl get pods -n tiny-cluster
kubectl logs -n tiny-cluster druid-metadata-postgresql-0
```

## 4. Install the Druid operator
a. Clone the repo and switch to `kick-tires` branch.
```shell script
git clone https://github.com/confluentinc/druid-operator.git
git checkout cc-druid-operator
```

b. Create the required k8s resources.
```shell script
kubectl create --namespace tiny-cluster -f deploy/service_account.yaml
# Setup RBAC
kubectl create --namespace tiny-cluster -f deploy/role.yaml
kubectl create --namespace tiny-cluster -f deploy/role_binding.yaml
# Setup the CRD
kubectl create --namespace tiny-cluster -f deploy/crds/druid.apache.org_druids.yaml
```
c. Deploy the druid-operator
```shell script
eval $(minikube -p minikube docker-env)

make docker-build
kubectl apply --namespace tiny-cluster -f deploy/operator.yaml
```
d. Check the deployed druid-operator. Make sure the pods are up and running.
```shell script
kubectl describe --namespace tiny-cluster deployment druid-operator
kubectl get pods -n tiny-cluster
kubectl -n tiny-cluster logs -l app=druid-operator
```

## 4.Deploy druid cluster
 
a. Pull the image manually until I figure out how to add image secrets
```shell script
eval $(minikube -p minikube docker-env)
docker pull confluent-docker.jfrog.io/confluentinc/cc-druid:v1.202.0
```
b. Create the zookeeper cluster.
```shell script
kubectl apply --namespace tiny-cluster -f examples/tiny-cluster-zk.yaml
```
c. Create the Druid CRD.
```shell script
kubectl apply --namespace tiny-cluster -f examples/tiny-cluster.yaml
```
d. Verify that all pods are up and running
```shell script
kubectl get pods --namespace tiny-cluster
```
e. Access the Druid console at http://druid.tiny.test .

## 5. (Optional) Install turnilo (fork of Druid Pivot project, for slicing and dicing data)

a. Deploy and create ingress.
```shell script
kubectl run turnilo \
    --namespace tiny-cluster \
    --image=node:12 \
    --port=9090 \
    --command -- bash -c "npm install -g turnilo && turnilo --druid http://druid-tiny-cluster-routers:8088"

kubectl expose deployment turnilo --namespace tiny-cluster --port=9090

cat <<EOF | kubectl apply --namespace tiny-cluster -f -
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: turnilo-ingress
spec:
  rules:
  - host: turnilo.tiny.test
    http:
      paths:
      - path: /
        backend:
          serviceName: turnilo
          servicePort: 9090
EOF

```
b. Access the UI at http://turnlio.tiny.test


## 6. (Optional) Install Grafana (for creating dashboards)
```shell script

helm repo add bitnami https://charts.bitnami.com/bitnami
helm upgrade \
  --install grafana bitnami/grafana \
  --namespace tiny-cluster \
  --set plugins=abhisant-druid-datasource \
  --set admin.password=admin \
  --set ingress.enabled=true \
  --set "ingress.hosts[0].name"=grafana.tiny.test \
  --debug
```

# Development

## How to run the operator locally
```shell script
mkdir -p tmp && cd tmp
export RELEASE_VERSION=v0.11.0;curl -LO https://github.com/operator-framework/operator-sdk/releases/download/${RELEASE_VERSION}/operator-sdk-${RELEASE_VERSION}-x86_64-apple-darwin
ln -s operator-sdk-v0.11.0-x86_64-apple-darwin operator-sdk
chmod +x operator-sdk-v0.11.0-x86_64-apple-darwin
cd ..
make build
tmp/operator-sdk up local
```