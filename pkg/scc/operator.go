package scc

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"

	"github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io"
	"github.com/rancher/rancher/pkg/scc/controllers/registration"
	"github.com/rancher/rancher/pkg/scc/controllers/registrationrequest"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

type sccOperator struct {
	sccFactory *scc.Factory
	core       *v1core.Factory
}

func setup(wContext *wrangler.Context) (sccOperator, error) {
	restConfig := wContext.RESTConfig
	registrationSccFactory, err := scc.NewFactoryFromConfig(restConfig)
	if err != nil {
		return sccOperator{}, fmt.Errorf("error building scc controllers: %s", err.Error())
	}

	coreF, err := v1core.NewFactoryFromConfig(restConfig)
	if err != nil {
		return sccOperator{}, fmt.Errorf("error building core controllers: %s", err.Error())
	}

	return sccOperator{
		sccFactory: registrationSccFactory,
		core:       coreF,
	}, nil
}

// maybeFirstInit will check if the initial `RegistrationRequest` seeding values exist
// and if they need to be processed into a new `RegistrationRequest` (used during first boot ever)
func (so *sccOperator) maybeFirstInit() error {
	if strings.EqualFold(settings.FirstSCCStart.Get(), "false") {
		return nil
	}

	// Check if the `cattle-system:initial-scc-registration` ConfigMap exists
	// If it does not, then we simply proceed and mark the setting as false
	configMap, err := so.core.Core().V1().ConfigMap().Get("cattle-system", "initial-scc-registration", metav1.GetOptions{})
	if err == nil {
		// Verify the expected fields are on the config map
		mode, ok := configMap.Data["mode"]
		// TODO bail here if OK is bad
		if mode
	}

	// At very end, we will set it to false so this doesn't run again
	if !strings.EqualFold(settings.FirstSCCStart.Get(), "false") {
		if err := settings.FirstSCCStart.Set("false"); err != nil {
			return err
		}
	}
}

func Setup(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	initOperator, err := setup(wContext)
	if err != nil {
		return fmt.Errorf("error setting up scc operator: %s", err.Error())
	}

	err = initOperator.maybeFirstInit()
	if err != nil {
		return fmt.Errorf("error creating first-start `RegistrationRequest`: %s", err.Error())
	}

	// TODO: Track if this cluster has had registration operator started ever
	// TODO: On first boot, check/fetch the ConfigMap/Secrets to populate a RegistrationRequest
	// This should be skipped on subsequent starts of the operator

	// TODO register controllers here
	logrus.Info("[scc-operator] Setup controllers here")
	registrationrequest.Register(
		ctx,
		initOperator.sccFactory.Scc().V1().RegistrationRequest(),
		initOperator.core.Core().V1().ConfigMap(),
		initOperator.core.Core().V1().Secret(),
	)
	registration.Register(
		ctx,
		initOperator.sccFactory.Scc().V1().Registration(),
	)

	// TODO: Some where in operator, or in registration controller, the current Registration needs to be revalidated every 24 hours

	return nil
}
