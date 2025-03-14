package registrationrequest

import (
	"context"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
)

type handler struct {
	ctx                  context.Context
	registrationRequests registrationControllers.RegistrationRequestController
}

func Register(
	ctx context.Context,
	registrationRequests registrationControllers.RegistrationRequestController,
) {
	controller := &handler{
		ctx:                  ctx,
		registrationRequests: registrationRequests,
	}

	registrationRequests.OnChange(ctx, "registrationRequests", controller.OnRegistrationRequestChange)
}

func (h *handler) OnRegistrationRequestChange(_ string, registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	// TODO: handle the logic of what happens when one of these is created
	// Gist being:
	// 1. Verify RegistrationRequest is not already fulfilled or expired,
	// 2. Verify contents of RegistrationRequest (mode and creds),
	// 3. Attempt SCC phone home with Online mode OR process the offline mode
	// 4. At the end of either process the current RegistrationRequest is either: a) fulfilled, b) expired or c) failed (retry?)
	// 4+. If it was a success, then a new Registration is created (and the old one deleted or marked as not current?)
	return registrationRequest, nil
}
