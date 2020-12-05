/*

 */

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	druidv1alpha1 "github.com/druid-io/druid-operator/apis/druid/v1alpha1"
)

// DruidReconciler reconciles a Druid object
type DruidReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=druid.apache.org,resources=druids,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=druid.apache.org,resources=druids/status,verbs=get;update;patch

func (r *DruidReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("druid", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *DruidReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&druidv1alpha1.Druid{}).
		Complete(r)
}
