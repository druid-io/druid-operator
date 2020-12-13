package druid

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/druid-io/druid-operator/apis/druid/v1alpha1"

	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
)

func TestMakeStatefulSetForBroker(t *testing.T) {
	clusterSpec := readSampleDruidClusterSpec(t)

	nodeSpecUniqueStr := makeNodeSpecificUniqueString(clusterSpec, "brokers")
	nodeSpec := clusterSpec.Spec.Nodes["brokers"]

	actual, _ := makeStatefulSet(&nodeSpec, clusterSpec, makeLabelsForNodeSpec(&nodeSpec, clusterSpec, clusterSpec.Name, nodeSpecUniqueStr), nodeSpecUniqueStr, "blah", nodeSpecUniqueStr)
	addHashToObject(actual)

	expected := new(appsv1.StatefulSet)
	readAndUnmarshallResource("testdata/broker-statefulset.yaml", &expected, t)

	assertEquals(expected, actual, t)
}

func TestDeploymentForBroker(t *testing.T) {
	clusterSpec := readSampleDruidClusterSpec(t)

	nodeSpecUniqueStr := makeNodeSpecificUniqueString(clusterSpec, "brokers")
	nodeSpec := clusterSpec.Spec.Nodes["brokers"]

	actual, _ := makeDeployment(&nodeSpec, clusterSpec, makeLabelsForNodeSpec(&nodeSpec, clusterSpec, clusterSpec.Name, nodeSpecUniqueStr), nodeSpecUniqueStr, "blah", nodeSpecUniqueStr)
	addHashToObject(actual)

	expected := new(appsv1.Deployment)
	readAndUnmarshallResource("testdata/broker-deployment.yaml", &expected, t)

	assertEquals(expected, actual, t)
}

func TestMakePodDisruptionBudgetForBroker(t *testing.T) {
	clusterSpec := readSampleDruidClusterSpec(t)

	nodeSpecUniqueStr := makeNodeSpecificUniqueString(clusterSpec, "brokers")
	nodeSpec := clusterSpec.Spec.Nodes["brokers"]

	actual, _ := makePodDisruptionBudget(&nodeSpec, clusterSpec, makeLabelsForNodeSpec(&nodeSpec, clusterSpec, clusterSpec.Name, nodeSpecUniqueStr), nodeSpecUniqueStr)
	addHashToObject(actual)

	expected := new(v1beta1.PodDisruptionBudget)
	readAndUnmarshallResource("testdata/broker-pod-disruption-budget.yaml", &expected, t)

	assertEquals(expected, actual, t)
}

func TestMakeHeadlessService(t *testing.T) {
	clusterSpec := readSampleDruidClusterSpec(t)

	nodeSpecUniqueStr := makeNodeSpecificUniqueString(clusterSpec, "brokers")
	nodeSpec := clusterSpec.Spec.Nodes["brokers"]

	actual, _ := makeService(&nodeSpec.Services[0], &nodeSpec, clusterSpec, makeLabelsForNodeSpec(&nodeSpec, clusterSpec, clusterSpec.Name, nodeSpecUniqueStr), nodeSpecUniqueStr)
	addHashToObject(actual)

	expected := new(corev1.Service)
	readAndUnmarshallResource("testdata/broker-headless-service.yaml", &expected, t)

	assertEquals(expected, actual, t)
}

func TestMakeLoadBalancerService(t *testing.T) {
	clusterSpec := readSampleDruidClusterSpec(t)

	nodeSpecUniqueStr := makeNodeSpecificUniqueString(clusterSpec, "brokers")
	nodeSpec := clusterSpec.Spec.Nodes["brokers"]

	actual, _ := makeService(&nodeSpec.Services[1], &nodeSpec, clusterSpec, makeLabelsForNodeSpec(&nodeSpec, clusterSpec, clusterSpec.Name, nodeSpecUniqueStr), nodeSpecUniqueStr)
	addHashToObject(actual)

	expected := new(corev1.Service)
	readAndUnmarshallResource("testdata/broker-load-balancer-service.yaml", &expected, t)

	assertEquals(expected, actual, t)
}

func TestMakeConfigMap(t *testing.T) {
	clusterSpec := readSampleDruidClusterSpec(t)

	actual, _ := makeCommonConfigMap(clusterSpec, makeLabelsForDruid(clusterSpec.Name))
	addHashToObject(actual)

	expected := new(corev1.ConfigMap)
	readAndUnmarshallResource("testdata/common-config-map.yaml", &expected, t)

	assertEquals(expected, actual, t)
}

func TestMakeBrokerConfigMap(t *testing.T) {
	clusterSpec := readSampleDruidClusterSpec(t)

	nodeSpecUniqueStr := makeNodeSpecificUniqueString(clusterSpec, "brokers")
	nodeSpec := clusterSpec.Spec.Nodes["brokers"]

	actual, _ := makeConfigMapForNodeSpec(&nodeSpec, clusterSpec, makeLabelsForNodeSpec(&nodeSpec, clusterSpec, clusterSpec.Name, nodeSpecUniqueStr), nodeSpecUniqueStr)
	addHashToObject(actual)

	expected := new(corev1.ConfigMap)
	readAndUnmarshallResource("testdata/broker-config-map.yaml", &expected, t)

	assertEquals(expected, actual, t)
}

func readSampleDruidClusterSpec(t *testing.T) *v1alpha1.Druid {
	return readDruidClusterSpecFromFile(t, "testdata/druid-test-cr.yaml")
}

func readDruidClusterSpecFromFile(t *testing.T, filePath string) *v1alpha1.Druid {
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Errorf("Failed to read druid cluster spec")
	}

	clusterSpec := new(v1alpha1.Druid)
	err = yaml.Unmarshal(bytes, &clusterSpec)
	if err != nil {
		t.Errorf("Failed to unmarshall druid cluster spec: %v", err)
	}

	return clusterSpec
}

func readAndUnmarshallResource(file string, res interface{}, t *testing.T) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		t.Errorf("Failed to read file[%s]", file)
	}

	err = yaml.Unmarshal(bytes, res)
	if err != nil {
		t.Errorf("Failed to unmarshall resource from file[%s]", file)
	}
}

func assertEquals(expected, actual interface{}, t *testing.T) {
	if !reflect.DeepEqual(expected, actual) {
		t.Error("Match failed!.")
	}
}

func dumpResource(res interface{}, t *testing.T) {
	bytes, err := yaml.Marshal(res)
	if err != nil {
		t.Errorf("failed to marshall: %v", err)
	}
	fmt.Println(string(bytes))
}
