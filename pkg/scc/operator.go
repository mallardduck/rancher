package scc

import (
	"context"
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
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
	logrus.Info("SCC controller MaybeFirstInit")
	if strings.EqualFold(settings.SCCFirstStart.Get(), "false") {
		logrus.Warn("Skipping the SCC controller first start.")
		return nil
	}

	// TODO: On first boot, check/fetch the ConfigMap/Secrets to populate a RegistrationRequest
	// Check if the `cattle-system:initial-scc-registration` ConfigMap exists
	// If it does not, then we simply proceed and mark the setting as false
	configMap, err := so.core.Core().V1().ConfigMap().Get("cattle-system", "initial-scc-registration", metav1.GetOptions{})
	if err != nil {
		logrus.Warn("Cannot find initial-scc-registration configmap; it will be skipped")
		return nil
	}

	// Verify the expected fields are on the config map
	modeString, ok := configMap.Data["mode"]
	mode := v1.RegistrationMode(modeString)
	if !ok || !mode.Valid() {
		errorMsg := "cannot find the SCC registration initilizer configmap"
		if ok && !mode.Valid() {
			errorMsg = fmt.Sprintf("the configmap does not have a valid mode set")
		}
		// TODO bail here if OK is bad
		// Just unclear if we should: a) error, or b) silent error (letting `SCCFirstStart` get updated).
		logrus.Error(errorMsg)
		return fmt.Errorf(errorMsg)
	}

	newRegistrationRequest := &v1.RegistrationRequest{}
	newRegistrationRequest.Spec.Mode = mode

	credentialValueKey := ""
	if mode == v1.Online {
		credentialValueKey = "regCodeRef"
	} else {
		credentialValueKey = "certificateRef"
	}

	secretName, credOk := configMap.Data[credentialValueKey]
	if !credOk {
		// TODO bail here if OK is bad
		// Just unclear if we should: a) error, or b) silent error (letting `SCCFirstStart` get updated).
		errorMsg := fmt.Sprintf("cannot find the credential value key %s", credentialValueKey)
		logrus.Error(errorMsg)
		return fmt.Errorf(errorMsg)
	}

	secret, err := so.core.Core().V1().Secret().Get("cattle-system", secretName, metav1.GetOptions{})
	if err != nil {
		// TODO bail here if OK is bad
		// Just unclear if we should: a) error, or b) silent error (letting `SCCFirstStart` get updated).
		logrus.Error(err)
		return err
	}
	if secret != nil {
		newSecret := *secret
		if mode == "online" {
			regCode, hasRegCodeKey := newSecret.Data["regCode"]
			if !hasRegCodeKey {
				// TODO bail
				errorMsg := fmt.Sprintf("cannot find the expected regCode key on, %s", secret.Name)
				logrus.Error(errorMsg)
				return fmt.Errorf(errorMsg)
			}
			newRegistrationRequest.Spec.RegistrationCode = string(regCode)
		} else if mode == "offline" {
			regCode, hasCertKey := secret.Data["certificate"]
			if !hasCertKey {
				// TODO bail
				errorMsg := fmt.Sprintf("cannot find the expected certificate key on, %s", secret.Name)
				logrus.Error(errorMsg)
				return fmt.Errorf(errorMsg)
			}
			newRegistrationRequest.Spec.RegistrationCode = string(regCode)
		}
	}

	_, err = so.sccFactory.Scc().V1().RegistrationRequest().Create(newRegistrationRequest)
	if err != nil {
		logrus.Error("Cannot create registration request")
		return err
	}

	// At very end, we will set it to false so this doesn't run again
	if !strings.EqualFold(settings.SCCFirstStart.Get(), "false") {
		if err := settings.SCCFirstStart.Set("false"); err != nil {
			return err
		}
	}

	return nil
}

func Setup(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	logrus.Info("Starting SCC Operator")
	initOperator, err := setup(wContext)
	if err != nil {
		return fmt.Errorf("error setting up scc operator: %s", err.Error())
	}

	// This should be skipped on subsequent starts of the operator
	err = initOperator.maybeFirstInit()
	if err != nil {
		return fmt.Errorf("error creating first-start `RegistrationRequest`: %s", err.Error())
	}

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
