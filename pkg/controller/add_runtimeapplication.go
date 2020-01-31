package controller

import (
	"github.com/application-runtimes/operator/pkg/controller/runtimeapplication"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, runtimeapplication.Add)
}
