#!/bin/bash
#!/bin/sh
set -o errexit
# Get Kind
go install sigs.k8s.io/kind@v0.17.0
# minio statefulset name
MINIO_STS_NAME=minio1-ss-0
# druid namespace
NAMESPACE=druid
# fmt code
make fmt
# vet
make vet
# deploy kind
make kind
# build local docker druid operator image
make docker-build-local
# push to kind registery
make docker-push-local
# install druid-operator
make helm-install-druid-operator
# install minio-operator and tenant
make helm-minio-install
# hack for minio pod to get started
sleep 60
# wait for minio pods
kubectl rollout status sts $MINIO_STS_NAME -n ${NAMESPACE}  --timeout=60s
# output pods
kubectl get pods -n ${NAMESPACE}
# apply druid cr
kubectl apply -f e2e/configs/druid-cr.yaml -n ${NAMESPACE}
# hack for druid pods
sleep 30
# wait for druid pods
declare -a sts=`($(kubectl get sts -n ${NAMESPACE} -l app=${NAMESPACE} | awk '{ print $1 }' | awk '(NR>1)'))`
for s in ${sts[@]}; do
  echo $s 
  kubectl rollout status sts $s -n ${NAMESPACE}  --timeout=60s
done
