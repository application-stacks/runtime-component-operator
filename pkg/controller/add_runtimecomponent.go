package controller

import (
	"github.com/application-stacks/operator/pkg/controller/runtimecomponent"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, runtimecomponent.Add)
}
