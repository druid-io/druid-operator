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
	namespaces := getEnvAsSlice("DENY_LIST", nil, ",")

	for _, namespace := range namespaces {
		if e.Object.GetNamespace() == namespace {
			msg := fmt.Sprintf("druid operator will not re-concile namespace [%s], alter DENY_LIST to re-concile", e.Object.GetNamespace())
			logger.Info(msg)
			return false
		}
	}
	return true

}

// update() to filter update events
func (GenericPredicates) Update(e event.UpdateEvent) bool {
	namespaces := getEnvAsSlice("DENY_LIST", nil, ",")

	for _, namespace := range namespaces {
		if e.ObjectNew.GetNamespace() == namespace {
			msg := fmt.Sprintf("druid operator will not re-concile namespace [%s], alter DENY_LIST to re-concile", e.ObjectNew.GetNamespace())
			logger.Info(msg)
			return false
		}
	}
	return true

}

// Delete() to filter delete events
//func (GenericPredicates) Delete(e event.DeleteEvent) bool {
//	return true
//}

// Genreric() to filter generic events
//func (GenericPredicates)  Genreric(e event.GenericEvent) bool {
//	return true
//}
