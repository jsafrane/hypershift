package util

import (
	hyperapi "github.com/openshift/hypershift/support/api"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
)

var (
	// scheme used by client in e2e test suite.
	// This scheme was born out of the requirement of certain
	// GVKs in the testing environment that are not required by
	// the client used by a running HyperShift instance.
	scheme = hyperapi.Scheme
)

func init() {
	operatorsv1.AddToScheme(scheme)
	operatorsv1alpha1.AddToScheme(scheme)
}
