package druid

import (
	"github.com/druid-io/druid-operator/pkg/apis/druid/v1alpha1"
	"encoding/json"
	"k8s.io/api/core/v1"
	"testing"
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

	clusterSpec := v1alpha1.DruidClusterSpec{}
	if err := json.Unmarshal(js, &clusterSpec); err != nil {
		t.Errorf("Failed to unmarshall[%v]", err)
	}

	if x := firstNonNilValue(clusterSpec.Nodes["brokers"].SecurityContext, clusterSpec.SecurityContext).(*v1.PodSecurityContext); *x.RunAsUser != 106 {
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
