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

}
