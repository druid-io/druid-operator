package druid

import (
	"context"
	"fmt"

	"github.com/druid-io/druid-operator/apis/druid/v1alpha1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DruidNodeStatus string

const (
	resourceCreated DruidNodeStatus = "CREATED"
	resourceUpdated DruidNodeStatus = "UPDATED"
)

const (
	DruidNodeUpdateFail            string = "UPDATE_FAIL"
	DruidNodeUpdateSuccess         string = "UPDATE_SUCCESS"
	DruidNodeRollingDeploymentWait string = "ROLLING_DEPLOYMENT_WAIT"
	DruidNodeDeleteFail            string = "DELETE_FAIL"
	DruidNodeDeleteSuccess         string = "DELETE_SUCCESS"
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
	List(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() objectList, ListObjFn func(obj runtime.Object) []object) ([]object, error)
	Get(ctx context.Context, sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) (object, error)
}

// Writer Interface
type Writer interface {
	Delete(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, obj object, deleteOptions ...client.DeleteOption) error
	Create(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, obj object) (DruidNodeStatus, error)
	Update(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, obj object) (DruidNodeStatus, error)
	Patch(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, obj object, status bool, patch client.Patch) error
}

// WriterFuncs struct
type WriterFuncs struct{}

// ReaderFuncs struct
type ReaderFuncs struct{}

// Initalizie Reader
var readers Reader = ReaderFuncs{}

// Initalize Writer
var writers Writer = WriterFuncs{}

// Object Interface : Wrapper interface includes metav1 object and runtime object interface.
type object interface {
	metav1.Object
	runtime.Object
}

// Object List Interface : Wrapper interface includes metav1 List and runtime object interface.
type objectList interface {
	metav1.ListInterface
	runtime.Object
}

// Patch method shall patch the status of Obj or the status.
// Pass status as true to patch the object status.
// NOTE: Not logging on patch success, it shall keep logging on each reconcile
func (f WriterFuncs) Patch(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, obj object, status bool, patch client.Patch) error {
	if !status {
		if err := sdk.Patch(ctx, obj, patch); err != nil {
			e := fmt.Errorf("failed to patch for [%s:%s] due to [%s]", drd.Kind, drd.Name, err.Error())
			sendEvent(sdk, drd, v1.EventTypeWarning, DruidNodePatchFail, e.Error())
			logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
			return e
		}
	} else {
		if err := sdk.Status().Patch(ctx, obj, patch); err != nil {
			e := fmt.Errorf("failed to patch status object for [%s:%s] due to [%s]", drd.Kind, drd.Name, err.Error())
			sendEvent(sdk, drd, v1.EventTypeWarning, DruidNodePatchFail, e.Error())
			logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
			return e
		}
	}
	return nil
}

// Update Func shall update the Object
func (f WriterFuncs) Update(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, obj object) (DruidNodeStatus, error) {

	if err := sdk.Update(ctx, obj); err != nil {
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
func (f WriterFuncs) Create(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, obj object) (DruidNodeStatus, error) {

	if err := sdk.Create(ctx, obj); err != nil {
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
func (f WriterFuncs) Delete(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, obj object, deleteOptions ...client.DeleteOption) error {

	if err := sdk.Delete(ctx, obj, deleteOptions...); err != nil {
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
func (f ReaderFuncs) Get(ctx context.Context, sdk client.Client, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emptyObjFn func() object) (object, error) {
	obj := emptyObjFn()

	if err := sdk.Get(ctx, *namespacedName(nodeSpecUniqueStr, drd.Namespace), obj); err != nil {
		e := fmt.Errorf("failed to get [Object:%s] due to [%s]", nodeSpecUniqueStr, err.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		sendEvent(sdk, drd, v1.EventTypeWarning, DruidOjectGetFail, e.Error())
		return nil, e
	}
	return obj, nil
}

// List methods shall return the list of an object
func (f ReaderFuncs) List(ctx context.Context, sdk client.Client, drd *v1alpha1.Druid, selectorLabels map[string]string, emptyListObjFn func() objectList, ListObjFn func(obj runtime.Object) []object) ([]object, error) {
	listOpts := []client.ListOption{
		client.InNamespace(drd.Namespace),
		client.MatchingLabels(selectorLabels),
	}
	listObj := emptyListObjFn()

	if err := sdk.List(ctx, listObj, listOpts...); err != nil {
		e := fmt.Errorf("failed to list [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		sendEvent(sdk, drd, v1.EventTypeWarning, DruidObjectListFail, e.Error())
		logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
		return nil, e
	}

	return ListObjFn(listObj), nil
}
