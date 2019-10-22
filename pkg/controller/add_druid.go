package controller

import (
	"github.com/druid-io/druid-operator/pkg/controller/druid"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, druid.Add)
}
