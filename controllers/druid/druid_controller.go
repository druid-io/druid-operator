package druid

import (
	"context"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	druidv1alpha1 "github.com/druid-io/druid-operator/apis/druid/v1alpha1"
)

// DruidReconciler reconciles a Druid object
type DruidReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	// reconcile time duration, defaults to 10s
	ReconcileWait time.Duration
}

// +kubebuilder:rbac:groups=druid.apache.org,resources=druids,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=druid.apache.org,resources=druids/status,verbs=get;update;patch

func (r *DruidReconciler) Reconcile(ctx context.Context, request reconcile.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("druid", request.NamespacedName)

	// your logic here

	// Fetch the Druid instance
	instance := &druidv1alpha1.Druid{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if err := deployDruidCluster(r.Client, instance); err != nil {
		return ctrl.Result{}, err
	} else {
		return ctrl.Result{RequeueAfter: r.ReconcileWait}, nil
	}
}

func (r *DruidReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&druidv1alpha1.Druid{}).
		WithEventFilter(ignoreNamespacePredicate()).
		Complete(r)
}

func LookupReconcileTime() time.Duration {
	val, exists := os.LookupEnv("RECONCILE_WAIT")
	if !exists {
		return time.Second * 10
	} else {
		v, err := time.ParseDuration(val)
		if err != nil {
			logger.Error(err, err.Error())
			// Exit Program if not valid
			os.Exit(1)
		}
		return v
	}
}
