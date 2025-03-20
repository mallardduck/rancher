package registration

import (
	"context"
	"errors"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/sirupsen/logrus"

	sccRegistration "github.com/SUSE/connect-ng/pkg/registration"
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

func (h *handler) OnRegistrationChange(key string, registrationObj *v1.Registration) (*v1.Registration, error) {
	logrus.Infof("[scc.registrations-controller]: Received registrations %q", key)
	logrus.Info("[scc.registrations-controller]: registrations ", registrationObj)

	if registrationObj.Status.Mode == v1.Online {
		registrationObj, err := h.processOnlineRegistration(registrationObj)
		if err != nil {
			return h.setReconcilingCondition(registrationObj, err)
		}
	} else {
		registrationObj, err := h.processOfflineRegistration(registrationObj)
		if err != nil {
			return h.setReconcilingCondition(registrationObj, err)
		}
	}

	return registrationObj, nil
}

func (h *handler) setReconcilingCondition(registration *v1.Registration, originalErr error) (*v1.Registration, error) {
	logrus.Info("[scc.registration-controller]: set reconciling condition")
	logrus.Error(originalErr)

	// TODO: actually set the message to something that makes sense based on the error
	v1.RegistrationConditionFailed.SetStatusBool(registration, true)
	v1.RegistrationConditionFailed.SetError(registration, "", originalErr)

	registration, err := h.registrations.UpdateStatus(registration)
	if err != nil {
		return registration, errors.New(originalErr.Error() + err.Error())
	}

	return registration, originalErr
}

func (h *handler) processOnlineRegistration(registration *v1.Registration) (*v1.Registration, error) {
	// TODO grab credentials and reg from secret
	regcode := "INTERNAL-USE-ONLY-9e4b-3166"
	sccCredentials := suseconnect.SccCredentials{}
	sccConnection := suseconnect.DefaultRancherConnection(&sccCredentials)

	// TODO get rancher ID
	identifier := "rancher"
	version := "2.10.3"
	arch := "unknown"

	meta, root, rootErr := sccRegistration.Activate(sccConnection, identifier, version, arch, regcode)
	if rootErr != nil {
		return registration, rootErr
	}
	logrus.Info("[scc.registration-controller]: Successfully activated registration")
	logrus.Info(meta)
	logrus.Info(root)

	return registration, nil
}

func (h *handler) processOfflineRegistration(registration *v1.Registration) (*v1.Registration, error) {
	return registration, nil
}
