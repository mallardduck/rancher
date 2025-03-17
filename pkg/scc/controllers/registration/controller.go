package registration

import (
	"context"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/sirupsen/logrus"
)

type handler struct {
	ctx           context.Context
	registrations registrationControllers.RegistrationController
}

func Register(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
) {
	controller := &handler{
		ctx:           ctx,
		registrations: registrations,
	}

	registrations.OnChange(ctx, "registrations", controller.OnRegistrationChange)
}

func (h *handler) OnRegistrationChange(key string, registration *v1.Registration) (*v1.Registration, error) {
	logrus.Infof("[scc.registrations-controller]: Received registrations %q", key)
	logrus.Info("[scc.registrations-controller]: registrations ", registration)
	// TODO: handle registration changes - AFAIK the CRDs spec doesn't need user fields
	return registration, nil
}
