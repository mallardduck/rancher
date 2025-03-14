package registrationrequest

import (
	"context"

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
	logrus.Infof("[registrationrequest-controller]: Received registrationRequest %q", name)
	logrus.Info("[registrationrequest-controller]: RegistrationRequest ", registrationRequest)
	// TODO: handle the logic of what happens when one of these is created
	// Gist being:
	// 1. Verify RegistrationRequest is not already fulfilled or expired,
	// 2. Verify contents of RegistrationRequest (mode and creds),
	// 3. Attempt SCC phone home with Online mode OR process the offline mode
	// 4. At the end of either process the current RegistrationRequest is either: a) fulfilled, b) expired or c) failed (retry?)
	// 4+. If it was a success, then a new Registration is created (and the old one deleted or marked as not current?)
	return registrationRequest, nil
}
