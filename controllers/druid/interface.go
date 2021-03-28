package druid

import (
	"context"
	"fmt"

	"github.com/druid-io/druid-operator/apis/druid/v1alpha1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reader interface {
	List(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) ([]object, error)
	Get(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) (object, error)
}

type Writer interface {
	Delete(sdk client.Client, drd *v1alpha1.Druid, obj runtime.Object, deleteOptions ...client.DeleteOption) error
	Create(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error)
	Update(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error)
}

type WriterFuncs struct {
	deleteFunc func(sdk client.Client, drd *v1alpha1.Druid, obj runtime.Object) error
	createFunc func(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error)
	updateFunc func(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error)
}

type ReaderFuncs struct {
	listFunc func(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) ([]object, error)
	getFunc  func(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) (object, error)
}

var readers Reader = ReaderFuncs{}
var writers Writer = WriterFuncs{}

func (f WriterFuncs) Update(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error) {

	if err := sdk.Update(context.TODO(), obj); err != nil {
		e := fmt.Errorf("Failed to update [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
		logger.Error(e, e.Error(), "Current Object", stringifyForLogging(obj, drd), "Updated Object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeWarning, "UPDATE_FAIL", e.Error())
		return "", e
	} else {
		msg := fmt.Sprintf("Updated [%s:%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		logger.Info(msg, "Prev Object", stringifyForLogging(obj, drd), "Updated Object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeNormal, "UPDATE_SUCCESS", msg)
		return resourceUpdated, nil
	}

}

func (f WriterFuncs) Create(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error) {

	if err := sdk.Create(context.TODO(), obj); err != nil {
		e := fmt.Errorf("Failed to create [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
		logger.Error(e, e.Error(), "object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace, "errorType", apierrors.ReasonForError(err))
		sendEvent(sdk, drd, v1.EventTypeWarning, "CREATE_FAIL", e.Error())
		return "", e
	} else {
		msg := fmt.Sprintf("Created [%s:%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		logger.Info(msg, "Object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeNormal, "CREATE_SUCCESS", msg)
		return resourceCreated, nil
	}

}

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

func (f ReaderFuncs) Get(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) (object, error) {
	obj := emptyObjFn()
	if err := sdk.Get(context.TODO(), *namespacedName(nodeSpecUniqueStr, drd.Namespace), obj); err != nil {
		e := fmt.Errorf("failed to get [Object:%s] due to [%s]", nodeSpecUniqueStr, err.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeWarning, "GET_FAIL", e.Error())
		return nil, e
	}
	return obj, nil
}

func (f ReaderFuncs) List(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) ([]object, error) {
	listOpts := []client.ListOption{
		client.InNamespace(drd.Namespace),
		client.MatchingLabels(selectorLabels),
	}
	listObj := emptyListObjFn()
	if err := sdk.List(context.TODO(), listObj, listOpts...); err != nil {
		e := fmt.Errorf("failed to list [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		sendEvent(sdk, drd, v1.EventTypeWarning, "LIST_FAIL", e.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		return nil, e
	}

	return ListObjFn(listObj), nil
}
