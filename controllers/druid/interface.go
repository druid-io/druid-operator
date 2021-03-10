package druid

import (
	"context"
	"fmt"

	"github.com/druid-io/druid-operator/apis/druid/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reader interface {
	List(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) []object
	Get(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) object
}

type Funcs struct {
	listFunc func(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) []object
	getFunc  func(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) object
}

var readers Reader = Funcs{}

func (f Funcs) Get(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) object {
	obj := emptyObjFn()
	if err := sdk.Get(context.TODO(), *namespacedName(nodeSpecUniqueStr, drd.Namespace), obj); err != nil {
		e := fmt.Errorf("failed to get [StatefuleSet:%s] due to [%s]", nodeSpecUniqueStr, err.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeWarning, "GET_FAIL", e.Error())
		return nil
	}
	return obj
}

func (f Funcs) List(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) []object {
	listOpts := []client.ListOption{
		client.InNamespace(drd.Namespace),
		client.MatchingLabels(selectorLabels),
	}
	listObj := emptyListObjFn()
	if err := sdk.List(context.TODO(), listObj, listOpts...); err != nil {
		e := fmt.Errorf("failed to list [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		sendEvent(sdk, drd, v1.EventTypeWarning, "LIST_FAIL", e.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
	}

	return ListObjFn(listObj)
}
