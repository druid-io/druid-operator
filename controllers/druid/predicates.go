package druid

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// All methods to implement GenericPredicates type
// GenericPredicates to be passed to manager
type GenericPredicates struct {
	predicate.Funcs
}

// create() to filter create events
func (GenericPredicates) Create(e event.CreateEvent) bool {
	return IgnoreNamespacePredicate(e.Object)
}

// update() to filter update events
func (GenericPredicates) Update(e event.UpdateEvent) bool {
	return IgnoreNamespacePredicate(e.ObjectNew)

}

func IgnoreNamespacePredicate(obj object) bool {
	namespaces := getEnvAsSlice("DENY_LIST", nil, ",")

	for _, namespace := range namespaces {
		if obj.GetNamespace() == namespace {
			msg := fmt.Sprintf("druid operator will not re-concile namespace [%s], alter DENY_LIST to re-concile", obj.GetNamespace())
			logger.Info(msg)
			return false
		}
	}
	return true
}
