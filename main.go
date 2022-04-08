package main

import (
	"flag"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	druidv1alpha1 "github.com/druid-io/druid-operator/apis/druid/v1alpha1"
	"github.com/druid-io/druid-operator/controllers/druid"
	// +kubebuilder:scaffold:imports
)

var (
	scheme         = runtime.NewScheme()
	setupLog       = ctrl.Log.WithName("setup")
	watchNamespace = os.Getenv("WATCH_NAMESPACE")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = druidv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "e6946145.apache.org",
		Namespace:              os.Getenv("WATCH_NAMESPACE"),
		NewCache:               watchNamespaceCache(),
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (druid.NewDruidReconciler(mgr)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Druid")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func watchNamespaceCache() cache.NewCacheFunc {
	var managerWatchCache cache.NewCacheFunc
	if watchNamespace != "" {
		ns := strings.Split(watchNamespace, ",")
		for i := range ns {
			ns[i] = strings.TrimSpace(ns[i])
		}
		managerWatchCache = cache.MultiNamespacedCacheBuilder(ns)
		return managerWatchCache
	}
	managerWatchCache = (cache.NewCacheFunc)(nil)
	return managerWatchCache
}
