package druid

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ignoreNamespacePredicate(), called before re-concilation loop in watcher
func ignoreNamespacePredicate() predicate.Predicate {
	namespaces := getEnvAsSlice("DENY_LIST", nil, ",")
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			for _, namespace := range namespaces {
				if e.Object.GetNamespace() == namespace {
					msg := fmt.Sprintf("druid operator will not re-concile namespace [%s], alter DENY_LIST to re-concile", e.Object.GetNamespace())
					logger.Info(msg)
					return false
				}
			}
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			for _, namespace := range namespaces {
				if e.ObjectNew.GetNamespace() == namespace {
					msg := fmt.Sprintf("druid operator will not re-concile namespace [%s], alter DENY_LIST to re-concile", e.ObjectNew.GetNamespace())
					logger.Info(msg)
					return false
				}
			}
			return true
		},
	}
}
