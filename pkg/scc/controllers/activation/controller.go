package activation

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
	ctx         context.Context
	activations registrationControllers.ActivationController
	secrets     v1core.SecretController
	systemInfo  *util.RancherSystemInfo
}

func Register(
	ctx context.Context,
	activations registrationControllers.ActivationController,
	secrets v1core.SecretController,
	systemInfo *util.RancherSystemInfo,
) {
	controller := &handler{
		ctx:         ctx,
		activations: activations,
		secrets:     secrets,
		systemInfo:  systemInfo,
	}

	activations.OnChange(ctx, "activations", controller.OnActivationChange)
}

func (h *handler) OnActivationChange(key string, activation *v1.Activation) (*v1.Activation, error) {
	logrus.Infof("[scc.activations-controller]: Received activations %q", key)
	logrus.Info("[scc.activations-controller]: activations ", activation)

	if activation.Spec.CheckNow {
		if activation.Status.Mode == v1.Offline {
			updated := activation.DeepCopy()
			updated.Spec = v1.ActivationSpec{}
			updated, err := h.activations.Update(updated)
			if err != nil {
				return activation, err
			}

			// Also update the status to warn Offline users that `CheckNow` does nothing
			return updated, nil
		} else {
			updated := activation.DeepCopy()
			updated.Status.Valid = false
			updated, err := h.processOnlineRegistration(updated)
			if err != nil {
				return h.setReconcilingCondition(activation, err)
			}

			return updated, nil
		}
	}

	if activation.Status.Mode == v1.Online {
		registration, err := h.processOnlineRegistration(activation)
		if err != nil {
			return h.setReconcilingCondition(registration, err)
		}
	} else {
		registration, err := h.processOfflineRegistration(activation)
		if err != nil {
			return h.setReconcilingCondition(registration, err)
		}
	}

	return activation, nil
}

func (h *handler) setReconcilingCondition(activation *v1.Activation, originalErr error) (*v1.Activation, error) {
	logrus.Info("[scc.registration-controller]: set reconciling condition")
	logrus.Error(originalErr)

	// TODO: actually set the message to something that makes sense based on the error
	v1.ResourceConditionFailure.SetStatusBool(activation, true)
	v1.ResourceConditionFailure.SetError(activation, "", originalErr)

	registration, err := h.activations.UpdateStatus(activation)
	if err != nil {
		return registration, errors.New(originalErr.Error() + err.Error())
	}

	return registration, originalErr
}

func (h *handler) processOnlineRegistration(activation *v1.Activation) (*v1.Activation, error) {
	sccCredentials, credsErr := suseconnect.FetchSccCredentials(h.secrets)
	if credsErr != nil {
		return activation, credsErr
	}
	regCode, regErr := suseconnect.FetchSccRegistrationCodeFrom(h.secrets, activation.Status.RegistrationCodeSecretRef)
	if regErr != nil {
		return activation, regErr
	}

	sccConnection := suseconnect.DefaultRancherConnection(sccCredentials)

	// TODO get rancher ID
	identifier, version, arch := util.GetProductIdentifier("2.10.3")
	meta, root, rootErr := sccRegistration.Activate(sccConnection, identifier, version, arch, regCode)
	if rootErr != nil {
		return activation, rootErr
	}
	logrus.Info(meta)
	logrus.Info(root)

	systemInfo, err := h.systemInfo.PreparedForSCC()
	if err != nil {
		return activation, err
	}

	status, statusErr := sccRegistration.Status(sccConnection, h.systemInfo.ServerUrl(), systemInfo)
	if statusErr != nil {
		return activation, statusErr
	}

	if status == sccRegistration.Registered {
		logrus.Info("[scc.activation-controller]: Successfully registered activation")
		updated := activation.DeepCopy()
		updated.Status.LastValidatedTS = time.Now().UTC().Format(time.RFC3339)
		updated.Status.Valid = true
		updated.Spec = v1.ActivationSpec{}
		return h.activations.UpdateStatus(updated)
	}

	return activation, nil
}

func (h *handler) processOfflineRegistration(registration *v1.Activation) (*v1.Activation, error) {
	return registration, nil
}
