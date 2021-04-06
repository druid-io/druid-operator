package druid

import (
	"encoding/json"
	"testing"

	"github.com/druid-io/druid-operator/apis/druid/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func TestFirstNonNilValue(t *testing.T) {
	var js = []byte(`
    {
		"image": "himanshu01/druid:druid-0.12.0-1",
		"securityContext": { "fsGroup": 107, "runAsUser": 106 },
		"env": [{ "name": "k", "value": "v" }],
		"nodes":
		{
			"brokers": {
				"nodeType": "broker",
				"druid.port": 8080,
				"replicas": 2
			}
		}
    }`)

	clusterSpec := v1alpha1.DruidSpec{}
	if err := json.Unmarshal(js, &clusterSpec); err != nil {
		t.Errorf("Failed to unmarshall[%v]", err)
	}

	if x := firstNonNilValue(clusterSpec.Nodes["brokers"].PodSecurityContext, clusterSpec.PodSecurityContext).(*v1.PodSecurityContext); *x.RunAsUser != 106 {
		t.Fail()
	}

	if x := firstNonNilValue(clusterSpec.Nodes["brokers"].Env, clusterSpec.Env).([]v1.EnvVar); x[0].Name != "k" {
		t.Fail()
	}
}

func TestFirstNonEmptyStr(t *testing.T) {
	if firstNonEmptyStr("a", "b") != "a" {
		t.Fail()
	}

	if firstNonEmptyStr("", "b") != "b" {
		t.Fail()
	}
}

func TestContainsString(t *testing.T) {
	if ContainsString([]string{"a", "b"}, "a") == false {
		t.Fail()
	}
}

func TestRemoveString(t *testing.T) {
	rs := RemoveString([]string{"a", "b"}, "a")
	if linearsearch(rs, "a") {
		t.Fail()
	}

}

func linearsearch(list []string, key string) bool {
	for _, item := range list {
		if item == key {
			return true
		}
	}
	return false
}
