package scc

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io"
	"github.com/rancher/rancher/pkg/scc/controllers/registration"
	"github.com/rancher/rancher/pkg/scc/controllers/registrationrequest"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

type sccOperator struct {
	sccFactory *scc.Factory
}

func setup(wContext *wrangler.Context) (sccOperator, error) {
	restConfig := wContext.RESTConfig
	registrationSccFactory, err := scc.NewFactoryFromConfig(restConfig)
	if err != nil {
		return sccOperator{}, fmt.Errorf("error building scc controllers: %s", err.Error())
	}

	return sccOperator{
		sccFactory: registrationSccFactory,
	}, nil
}

func Setup(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	initOperator, err := setup(wContext)
	if err != nil {
		return fmt.Errorf("error setting up scc operator: %s", err.Error())
	}

	// TODO: Track if this cluster has had registration operator started ever
	// TODO: On first boot, check/fetch the ConfigMap/Secrets to populate a RegistrationRequest
	// This should be skipped on subsequent starts of the operator

	// TODO register controllers here
	logrus.Info("Setup controllers here")
	registrationrequest.Register(
		ctx,
		initOperator.sccFactory.Scc().V1().RegistrationRequest(),
	)
	registration.Register(
		ctx,
		initOperator.sccFactory.Scc().V1().Registration(),
	)

	// TODO: Some where in operator, or in registration controller, the current Registration needs to be revalidated every 24 hours

	return nil
}
