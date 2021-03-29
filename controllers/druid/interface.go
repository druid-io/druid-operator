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

const (
	resourceCreated = "CREATED"
	resourceUpdated = "UPDATED"
)

const (
	DruidNodeUpdateFail            string = "UPDATE_FAIL"
	DruidNodeUpdateSuccess         string = "UPDATE_SUCCESS"
	DruidNodeRollingDeploymentWait string = "ROLLING_DEPLOYMENT_WAIT"
	DruidNodeDeleteFail            string = "DELETE_FAIL"
	DruidNodeDeleteSuccess         string = "SUCCESS_FAIL"
	DruidNodeCreateSuccess         string = "CREATE_SUCCESS"
	DruidNodeCreateFail            string = "CREATE_FAIL"
	DruidNodePatchFail             string = "PATCH_FAIL"
	DruidSpecInvalid               string = "SPEC_INVALID"
	DruidNodeRunning               string = "RUNNING"
	DruidObjectListFail            string = "LIST_FAIL"
	DruidOjectGetFail              string = "GET_FAIL"
	DruidFinalizerSuccess          string = "TRIGGER_FINALIZER_SUCCESS"
	DruidFinalizer                 string = "TRIGGER_FINALIZER"
)

// Reader Interface
type Reader interface {
	List(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) ([]object, error)
	Get(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) (object, error)
}

// Writer Interface
type Writer interface {
	Delete(sdk client.Client, drd *v1alpha1.Druid, obj runtime.Object, deleteOptions ...client.DeleteOption) error
	Create(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error)
	Update(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error)
	StatusPatch(sdk client.Client, drd *v1alpha1.Druid, obj object, patch client.Patch) error
}

// WriterFuncs of type func
type WriterFuncs struct {
	deleteFunc      func(sdk client.Client, drd *v1alpha1.Druid, obj runtime.Object) error
	createFunc      func(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error)
	updateFunc      func(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error)
	statusPatchFunc func(sdk client.Client, drd *v1alpha1.Druid, obj object, patch client.Patch) error
}

// ReaderFuncs of type func
type ReaderFuncs struct {
	listFunc func(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) ([]object, error)
	getFunc  func(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) (object, error)
}

// Initalizie Reader
var readers Reader = ReaderFuncs{}

// Initalize Writer
var writers Writer = WriterFuncs{}

// StatusPatch method shall patch the status of Obj
// NOTE: Not logging on patch success, it shall keep logging on each reconcile
func (f WriterFuncs) StatusPatch(sdk client.Client, drd *v1alpha1.Druid, obj object, patch client.Patch) error {

	if err := sdk.Status().Patch(context.TODO(), obj, patch); err != nil {
		e := fmt.Errorf("failed to update status for [%s:%s] due to [%s]", drd.Kind, drd.Name, err.Error())
		sendEvent(sdk, drd, v1.EventTypeWarning, DruidNodePatchFail, e.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		return e
	}
	return nil
}

// Update Func shall update the Object
func (f WriterFuncs) Update(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error) {

	if err := sdk.Update(context.TODO(), obj); err != nil {
		e := fmt.Errorf("Failed to update [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
		logger.Error(e, e.Error(), "Current Object", stringifyForLogging(obj, drd), "Updated Object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeWarning, DruidNodeUpdateFail, e.Error())
		return "", e
	} else {
		msg := fmt.Sprintf("Updated [%s:%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		logger.Info(msg, "Prev Object", stringifyForLogging(obj, drd), "Updated Object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeNormal, DruidNodeUpdateSuccess, msg)
		return resourceUpdated, nil
	}

}

// Create methods shall create an object, and returns a string, error
func (f WriterFuncs) Create(sdk client.Client, drd *v1alpha1.Druid, obj object) (string, error) {

	if err := sdk.Create(context.TODO(), obj); err != nil {
		e := fmt.Errorf("Failed to create [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
		logger.Error(e, e.Error(), "object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace, "errorType", apierrors.ReasonForError(err))
		sendEvent(sdk, drd, v1.EventTypeWarning, DruidNodeCreateFail, e.Error())
		return "", e
	} else {
		msg := fmt.Sprintf("Created [%s:%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		logger.Info(msg, "Object", stringifyForLogging(obj, drd), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeNormal, DruidNodeCreateSuccess, msg)
		return resourceCreated, nil
	}

}

// Delete methods shall delete the object, deleteOptions is a variadic parameter to support various delete options such as cascade deletion.
func (f WriterFuncs) Delete(sdk client.Client, drd *v1alpha1.Druid, obj runtime.Object, deleteOptions ...client.DeleteOption) error {

	if err := sdk.Delete(context.TODO(), obj, deleteOptions...); err != nil {
		e := fmt.Errorf("Error deleting object [%s] in namespace [%s] due to [%s]", obj.GetObjectKind().GroupVersionKind().Kind, drd.Namespace, err.Error())
		sendEvent(sdk, drd, v1.EventTypeWarning, DruidNodeDeleteFail, e.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		return e
	} else {
		msg := fmt.Sprintf("Successfully deleted object [%s] in namespace [%s]", obj.GetObjectKind().GroupVersionKind().Kind, drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeNormal, DruidNodeDeleteSuccess, msg)
		logger.Info(msg, "name", drd.Name, "namespace", drd.Namespace)
		return nil
	}
}

// Get methods shall the get the object.
func (f ReaderFuncs) Get(sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) (object, error) {
	obj := emptyObjFn()

	if err := sdk.Get(context.TODO(), *namespacedName(nodeSpecUniqueStr, drd.Namespace), obj); err != nil {
		e := fmt.Errorf("failed to get [Object:%s] due to [%s]", nodeSpecUniqueStr, err.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeWarning, DruidOjectGetFail, e.Error())
		return nil, e
	}
	return obj, nil
}

// List methods shall return the list of an object
func (f ReaderFuncs) List(sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() runtime.Object, ListObjFn func(obj runtime.Object) []object) ([]object, error) {
	listOpts := []client.ListOption{
		client.InNamespace(drd.Namespace),
		client.MatchingLabels(selectorLabels),
	}
	listObj := emptyListObjFn()

	if err := sdk.List(context.TODO(), listObj, listOpts...); err != nil {
		e := fmt.Errorf("failed to list [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		sendEvent(sdk, drd, v1.EventTypeWarning, DruidObjectListFail, e.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		return nil, e
	}

	return ListObjFn(listObj), nil
}
