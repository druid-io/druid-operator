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

type Writer interface {
	Delete(sdk client.Client, drd *v1alpha1.Druid, obj runtime.Object, deleteOptions ...client.DeleteOption) error
}

type WriterFuncs struct {
	deleteFunc func(sdk client.Client, drd *v1alpha1.Druid, obj runtime.Object) error
}

type ReaderFuncs struct {
	listFunc func(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) []object
	getFunc  func(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) object
}

var readers Reader = ReaderFuncs{}
var writers Writer = WriterFuncs{}

func (f WriterFuncs) Delete(sdk client.Client, drd *v1alpha1.Druid, obj runtime.Object, deleteOptions ...client.DeleteOption) error {

	if err := sdk.Delete(context.TODO(), obj, deleteOptions...); err != nil {
		e := fmt.Errorf("Error deleting object [%s] in namespace [%s] due to [%s]", obj.GetObjectKind().GroupVersionKind().Kind, drd.Namespace, err.Error())
		sendEvent(sdk, drd, v1.EventTypeWarning, "DELETE_FAIL", e.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		return e
	} else {
		msg := fmt.Sprintf("Successfully deleted object [%s] in namespace [%s]", obj.GetObjectKind().GroupVersionKind().Kind, drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeNormal, "DELETE_SUCCESS", msg)
		logger.Info(msg, "name", drd.Name, "namespace", drd.Namespace)
	}

	return nil
}

func (f ReaderFuncs) Get(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) object {
	obj := emptyObjFn()
	if err := sdk.Get(context.TODO(), *namespacedName(nodeSpecUniqueStr, drd.Namespace), obj); err != nil {
		e := fmt.Errorf("failed to get [Object:%s] due to [%s]", nodeSpecUniqueStr, err.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeWarning, "GET_FAIL", e.Error())
		return nil
	}
	return obj
}

func (f ReaderFuncs) List(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) []object {
	listOpts := []client.ListOption{
		client.InNamespace(drd.Namespace),
		client.MatchingLabels(selectorLabels),
	}
	listObj := emptyListObjFn()
	if err := sdk.List(context.TODO(), listObj, listOpts...); err != nil {
		e := fmt.Errorf("failed to list [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		sendEvent(sdk, drd, v1.EventTypeWarning, "LIST_FAIL", e.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		return nil
	}

	return ListObjFn(listObj)
}
