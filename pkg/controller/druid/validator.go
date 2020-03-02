package druid

import (
	"github.com/druid-io/druid-operator/pkg/apis/druid/v1alpha1"
)

// Validator defines validator struct
type Validator struct {
	Validated    bool
	ErrorMessage string
}

// Validate Druid
func (v *Validator) Validate(cr *v1alpha1.Druid) {
	v.Validated = true

	if cr.Spec.CommonRuntimeProperties == "" {
		v.ErrorMessage = v.ErrorMessage + "CommonRuntimeProperties missing from Druid Cluster Spec\n"
		v.Validated = false
	}

	if cr.Spec.CommonConfigMountPath == "" {
		v.ErrorMessage = v.ErrorMessage + "CommonConfigMountPath missing from Druid Cluster Spec\n"
		v.Validated = false
	}

	if cr.Spec.StartScript == "" {
		v.ErrorMessage = v.ErrorMessage + "StartScript missing from Druid Cluster Spec\n"
		v.Validated = false
	}

	if cr.Spec.Image == "" {
		v.ErrorMessage = v.ErrorMessage + "Image missing from Druid Cluster Spec\n"
		v.Validated = false
	}

	for _, n := range cr.Spec.Nodes {
		if n.NodeType == "" {
			v.ErrorMessage = v.ErrorMessage + "NodeType missing from Druid Node Spec\n"
			v.Validated = false
		}

		if n.Replicas < 1 {
			v.ErrorMessage = v.ErrorMessage + "Minimum of one Replicas needed in Druid Node Spec\n"
			v.Validated = false
		}

		if n.RuntimeProperties == "" {
			v.ErrorMessage = v.ErrorMessage + "RuntimeProperties missing in Druid Node Spec\n"
			v.Validated = false
		}

		if n.NodeConfigMountPath == "" {
			v.ErrorMessage = v.ErrorMessage + "NodeConfigMountPath missing in Druid Node Spec\n"
			v.Validated = false
		}
	}
}
