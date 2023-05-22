package druid

import (
	"testing"

	"github.com/datainfrahq/druid-operator/apis/druid/v1alpha1"
)

func TestIt(t *testing.T) {
	v := v1alpha1.ZookeeperSpec{
		Type: "default",
		Spec: []byte(`{ "properties": "my-zookeeper-config" }`),
	}

	if zm, err := createZookeeperManager(&v); err != nil {
		t.Error(err.Error())
	} else {
		if zm.Configuration() != "my-zookeeper-config" {
			t.Errorf("Error: Expected[%s], Actual[%s]", "my-zookeeper-config", zm.Configuration())
		}
	}
}
