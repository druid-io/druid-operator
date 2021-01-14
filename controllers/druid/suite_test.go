package druid

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	druidv1alpha1 "github.com/druid-io/druid-operator/apis/druid/v1alpha1"
	// +kubebuilder:scaffold:imports
)

type TestK8sEnvCtx struct {
	restConfig *rest.Config
	k8sManager manager.Manager
	env        *envtest.Environment
}

func setupK8Evn(t *testing.T, testK8sCtx *TestK8sEnvCtx) {
	ctrl.SetLogger(zap.New())
	testK8sCtx.env = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{filepath.Join("..", "..", "deploy", "crds", "druid.apache.org_druids.yaml")},
		},
	}

	config, err := testK8sCtx.env.Start()
	require.NoError(t, err)

	testK8sCtx.restConfig = config
}

func destroyK8Env(t *testing.T, testK8sCtx *TestK8sEnvCtx) {
	require.NoError(t, testK8sCtx.env.Stop())
}

func TestAPIs(t *testing.T) {
	testK8sCtx := &TestK8sEnvCtx{}

	setupK8Evn(t, testK8sCtx)
	defer destroyK8Env(t, testK8sCtx)

	err := druidv1alpha1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	testK8sCtx.k8sManager, err = ctrl.NewManager(testK8sCtx.restConfig, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	require.NoError(t, err)

	setupDruidOperator(t, testK8sCtx)

	go func() {
		err = testK8sCtx.k8sManager.Start(ctrl.SetupSignalHandler())
		if err != nil {
			fmt.Printf("problem starting mgr [%v] \n", err)
		}
		require.NoError(t, err)
	}()

	testDruidOperator(t, testK8sCtx)
}

func setupDruidOperator(t *testing.T, testK8sCtx *TestK8sEnvCtx) {
	err := (&DruidReconciler{
		Client:        testK8sCtx.k8sManager.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("Druid"),
		Scheme:        testK8sCtx.k8sManager.GetScheme(),
		ReconcileWait: LookupReconcileTime(),
	}).SetupWithManager(testK8sCtx.k8sManager)

	require.NoError(t, err)
}

func testDruidOperator(t *testing.T, testK8sCtx *TestK8sEnvCtx) {
	druidCR := readDruidClusterSpecFromFile(t, "testdata/druid-smoke-test-cluster.yaml")

	k8sClient := testK8sCtx.k8sManager.GetClient()

	err := k8sClient.Create(context.TODO(), druidCR)
	require.NoError(t, err)

	err = retry(func() error {
		druid := &druidv1alpha1.Druid{}

		err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: druidCR.Name, Namespace: druidCR.Namespace}, druid)
		if err != nil {
			return err
		}

		expectedConfigMaps := []string{
			fmt.Sprintf("druid-%s-brokers-config", druidCR.Name),
			fmt.Sprintf("druid-%s-coordinators-config", druidCR.Name),
			fmt.Sprintf("druid-%s-historicals-config", druidCR.Name),
			fmt.Sprintf("druid-%s-routers-config", druidCR.Name),
			fmt.Sprintf("%s-druid-common-config", druidCR.Name),
		}
		if !areStringArraysEqual(druid.Status.ConfigMaps, expectedConfigMaps) {
			return errors.New(fmt.Sprintf("Failed to get expected ConfigMaps, got [%v]", druid.Status.ConfigMaps))
		}

		expectedServices := []string{
			fmt.Sprintf("druid-%s-brokers", druidCR.Name),
			fmt.Sprintf("druid-%s-coordinators", druidCR.Name),
			fmt.Sprintf("druid-%s-historicals", druidCR.Name),
			fmt.Sprintf("druid-%s-routers", druidCR.Name),
		}
		if !areStringArraysEqual(druid.Status.Services, expectedServices) {
			return errors.New(fmt.Sprintf("Failed to get expected Services, got [%v]", druid.Status.Services))
		}

		expectedStatefulSets := []string{
			fmt.Sprintf("druid-%s-coordinators", druidCR.Name),
			fmt.Sprintf("druid-%s-historicals", druidCR.Name),
			fmt.Sprintf("druid-%s-routers", druidCR.Name),
		}
		if !areStringArraysEqual(druid.Status.StatefulSets, expectedStatefulSets) {
			return errors.New(fmt.Sprintf("Failed to get expected StatefulSets, got [%v]", druid.Status.StatefulSets))
		}

		expectedDeployments := []string{
			fmt.Sprintf("druid-%s-brokers", druidCR.Name),
		}
		if !areStringArraysEqual(druid.Status.Deployments, expectedDeployments) {
			return errors.New(fmt.Sprintf("Failed to get expected Deployments, got [%v]", druid.Status.Deployments))
		}

		expectedPDBs := []string{
			fmt.Sprintf("druid-%s-brokers", druidCR.Name),
		}
		if !areStringArraysEqual(druid.Status.PodDisruptionBudgets, expectedPDBs) {
			return errors.New(fmt.Sprintf("Failed to get expected PDBs, got [%v]", druid.Status.PodDisruptionBudgets))
		}

		expectedHPAs := []string{
			fmt.Sprintf("druid-%s-brokers", druidCR.Name),
		}
		if !areStringArraysEqual(druid.Status.HPAutoScalers, expectedHPAs) {
			return errors.New(fmt.Sprintf("Failed to get expected HPAs, got [%v]", druid.Status.HPAutoScalers))
		}

		expectedIngress := []string{
			fmt.Sprintf("druid-%s-routers", druidCR.Name),
		}
		if !areStringArraysEqual(druid.Status.Ingress, expectedIngress) {
			return errors.New(fmt.Sprintf("Failed to get expected Ingress, got [%v]", druid.Status.Ingress))
		}

		return nil
	}, time.Millisecond*250, time.Second*45)

	require.NoError(t, err)
}

func areStringArraysEqual(a1, a2 []string) bool {
	if len(a1) == len(a2) {
		for i, v := range a1 {
			if v != a2[i] {
				return false
			}
		}
	} else {
		return false
	}
	return true
}

func retry(fn func() error, retryWait, timeOut time.Duration) error {
	timeOutTimestamp := time.Now().Add(timeOut)

	for {
		if err := fn(); err != nil {
			if time.Now().Before(timeOutTimestamp) {
				time.Sleep(retryWait)
			} else {
				return err
			}
		} else {
			return nil
		}
	}
}
