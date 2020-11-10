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
				if e.Meta.GetNamespace() == namespace {
					msg := fmt.Sprintf("druid operator will not re-concile namespace [%s], alter DENY_LIST to re-concile", e.Meta.GetNamespace())
					log.Info(msg)
					return false
				}
			}
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			for _, namespace := range namespaces {
				if e.MetaNew.GetNamespace() == namespace {
					msg := fmt.Sprintf("druid operator will not re-concile namespace [%s], alter DENY_LIST to re-concile", e.MetaNew.GetNamespace())
					log.Info(msg)
					return false
				}
			}
			return true
		},
	}
}
