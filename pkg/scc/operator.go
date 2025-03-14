package scc

import (
	"context"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

func Register(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	// TODO: Track if this cluster has had registration operator started ever
	// TODO: On first boot, check/fetch the ConfigMap/Secrets to populate a RegistrationRequest
	// This should be skipped on subsequent starts of the cluster

	// TODO register controllers here
	logrus.Info("Register controllers here")

	// TODO: Some where in operator, or in registration controller, the current Registration needs to be revalidated every 24 hours

	return nil
}
