package registrationrequest

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/scc/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"

	"github.com/sirupsen/logrus"
)

type handler struct {
	ctx                  context.Context
	registrationRequests registrationControllers.RegistrationRequestController
	configMaps           v1core.ConfigMapController
	secrets              v1core.SecretController
}

func Register(
	ctx context.Context,
	registrationRequests registrationControllers.RegistrationRequestController,
	configMaps v1core.ConfigMapController,
	secrets v1core.SecretController,
) {
	controller := &handler{
		ctx:                  ctx,
		registrationRequests: registrationRequests,
		configMaps:           configMaps,
		secrets:              secrets,
	}

	registrationRequests.OnChange(ctx, "registrationRequests", controller.OnRegistrationRequestChange)
}

func (h *handler) OnRegistrationRequestChange(name string, registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	if registrationRequest == nil {
		return nil, nil
	}

	logrus.Infof("[scc.registrationrequest-controller]: Received registrationRequest %q", name)
	logrus.Info("[scc.registrationrequest-controller]: RegistrationRequest ", registrationRequest)
	// TODO: handle the logic of what happens when one of these is created
	// Gist being:
	// 1. Verify RegistrationRequest is not already fulfilled or expired,
	if registrationRequest.Status.RequestProcessedTS != "" {
		logrus.Info("[scc.registrationrequest-controller]: RegistrationRequest already processed")
		return registrationRequest, nil
	}
	// 2. Verify contents of RegistrationRequest (mode and creds),
	if registrationRequest.Spec.Mode == v1.Online {
		err := h.processOnlineRegistration(registrationRequest)
		if err != nil {
			return nil, err
		}
	} else {
		err := h.processOfflineRegistration(registrationRequest)
		if err != nil {
			return nil, err
		}
	}

	// 4. At the end of either process the current RegistrationRequest is either: a) fulfilled, b) expired or c) failed (retry?)
	// 4+. If it was a success, then a new Registration is created (and the old one deleted or marked as not current?)
	return registrationRequest, nil
}

func (h *handler) processOnlineRegistration(registrationRequest *v1.RegistrationRequest) error {
	logrus.Info("[scc.registrationrequest-controller]: online mode ")
	// 1. Verify the secret ref name
	secretName := util.RegCodeSecretName
	if registrationRequest.Spec.RegistrationCodeSecretRef != nil {
		secretName = registrationRequest.Spec.RegistrationCodeSecretRef.Name
	}

	// 2. Verify RegistrationRequest creds,
	regSecret, err := h.secrets.Get("", secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	_, ok := regSecret.Data[util.RegCodeSecretKey]
	if !ok {
		return errors.New(fmt.Sprintf("registration secret `%s` does not contain expected data `%s`", secretName, util.RegCodeSecretKey))
	}

	// 2. Attempt SCC phone home with Online mode OR process the offline mode

	return nil
}

func (h *handler) processOfflineRegistration(registrationRequest *v1.RegistrationRequest) error {
	// TODO implement offline mechanism
	logrus.Info("[scc.registrationrequest-controller]: offline mode ")
	return nil
}
