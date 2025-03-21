package registration

import (
	"context"
	"errors"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/util"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"time"

	sccRegistration "github.com/SUSE/connect-ng/pkg/registration"
)

type handler struct {
	ctx           context.Context
	registrations registrationControllers.RegistrationController
	secrets       v1core.SecretController
	systemInfo    *util.RancherSystemInfo
}

func Register(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
	secrets v1core.SecretController,
	systemInfo *util.RancherSystemInfo,
) {
	controller := &handler{
		ctx:           ctx,
		registrations: registrations,
		secrets:       secrets,
		systemInfo:    systemInfo,
	}

	registrations.OnChange(ctx, "registrations", controller.OnRegistrationChange)
}

func (h *handler) OnRegistrationChange(key string, registration *v1.Registration) (*v1.Registration, error) {
	logrus.Infof("[scc.registrations-controller]: Received registrations %q", key)
	logrus.Info("[scc.registrations-controller]: registrations ", registration)

	if registration.Spec.CheckNow {
		if registration.Status.Mode == v1.Offline {
			updated := registration.DeepCopy()
			updated.Spec = v1.RegistrationSpec{}
			updated, err := h.registrations.Update(updated)
			if err != nil {
				return registration, err
			}

			// Also update the status to warn Offline users that `CheckNow` does nothing
			return updated, nil
		} else {
			updated := registration.DeepCopy()
			updated.Status.Valid = false
			updated, err := h.processOnlineRegistration(updated)
			if err != nil {
				return h.setReconcilingCondition(registration, err)
			}

			return updated, nil
		}
	}

	if registration.Status.Mode == v1.Online {
		registration, err := h.processOnlineRegistration(registration)
		if err != nil {
			return h.setReconcilingCondition(registration, err)
		}
	} else {
		registration, err := h.processOfflineRegistration(registration)
		if err != nil {
			return h.setReconcilingCondition(registration, err)
		}
	}

	return registration, nil
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
	sccCredentials, credsErr := suseconnect.FetchSccCredentials(h.secrets)
	if credsErr != nil {
		return registration, credsErr
	}
	regCode, regErr := suseconnect.FetchSccRegistrationCodeFrom(h.secrets, registration.Status.RegistrationCodeSecretRef)
	if regErr != nil {
		return registration, regErr
	}

	sccConnection := suseconnect.DefaultRancherConnection(sccCredentials)

	// TODO get rancher ID
	identifier, version, arch := util.GetProductIdentifier("2.10.3")
	meta, root, rootErr := sccRegistration.Activate(sccConnection, identifier, version, arch, regCode)
	if rootErr != nil {
		return registration, rootErr
	}
	logrus.Info(meta)
	logrus.Info(root)

	systemInfo, err := h.systemInfo.PreparedForSCC()
	if err != nil {
		return registration, err
	}

	status, statusErr := sccRegistration.Status(sccConnection, h.systemInfo.ServerUrl(), systemInfo)
	if statusErr != nil {
		return registration, statusErr
	}

	if status == sccRegistration.Registered {
		logrus.Info("[scc.registration-controller]: Successfully registered registration")
		updatedRegistration := registration.DeepCopy()
		updatedRegistration.Status.LastValidatedTS = time.Now().UTC().Format(time.RFC3339)
		updatedRegistration.Status.Valid = true
		updatedRegistration.Spec = v1.RegistrationSpec{}
		return h.registrations.UpdateStatus(updatedRegistration)
	}

	return registration, nil
}

func (h *handler) processOfflineRegistration(registration *v1.Registration) (*v1.Registration, error) {
	return registration, nil
}
