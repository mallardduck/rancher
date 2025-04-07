package activation

import (
	"context"
	"errors"
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/util"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"time"

	sccRegistration "github.com/SUSE/connect-ng/pkg/registration"
)

type handler struct {
	ctx            context.Context
	activations    registrationControllers.ActivationController
	secrets        v1core.SecretController
	sccCredentials *credentials.CredentialSecretsAdapter
	systemInfo     *util.RancherSystemInfo
}

func Register(
	ctx context.Context,
	activations registrationControllers.ActivationController,
	secrets v1core.SecretController,
	systemInfo *util.RancherSystemInfo,
) {
	controller := &handler{
		ctx:            ctx,
		activations:    activations,
		secrets:        secrets,
		sccCredentials: credentials.New(secrets),
		systemInfo:     systemInfo,
	}

	activations.OnChange(ctx, "activations", controller.OnActivationChange)
	// TODO: EnqueueAfter - revalidate every 24 hours
	// Ex: https://github.com/rancher/rancher/blob/d6b40c3acd945f0c8fe463ff96d144561c9640c3/pkg/controllers/dashboard/helm/repo.go#L95
}

func (h *handler) OnActivationChange(key string, activation *v1.Activation) (*v1.Activation, error) {
	if activation == nil {
		return nil, fmt.Errorf("received nil activation")
	}

	logrus.Infof("[scc.activations-controller]: Received activations %q", key)
	logrus.Info("[scc.activations-controller]: activations ", activation)

	var lastValidatedTS time.Time
	if activation.Status.LastValidatedTS != "" {
		lastValidatedTS, _ = time.Parse(time.RFC3339, activation.Status.LastValidatedTS)
	}

	if activation.Spec.CheckNow && !lastValidatedTS.IsZero() {
		if activation.Status.Mode == v1.Offline {
			updated := activation.DeepCopy()
			// TODO: Also update the status to warn Offline users that `CheckNow` does nothing
			// Better alternative, webhook prevent updates if mode=offline
			updated.Spec = v1.ActivationSpec{}
			return h.activations.Update(updated)
		} else {
			updated := activation.DeepCopy()
			updated.Spec = v1.ActivationSpec{}
			updated.Status.Valid = false
			updated, err := h.processOnlineActivation(updated)
			if err != nil {
				return h.setReconcilingCondition(activation, err)
			}

			return updated, nil
		}
	}

	if !lastValidatedTS.IsZero() && time.Now().Sub(lastValidatedTS) < time.Hour {
		return activation, nil
	}

	if activation.Status.Mode == v1.Online {
		registration, err := h.processOnlineActivation(activation)
		if err != nil {
			return h.setReconcilingCondition(registration, err)
		}
	} else {
		registration, err := h.processOfflineActivation(activation)
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

func (h *handler) processOnlineActivation(activation *v1.Activation) (*v1.Activation, error) {
	_ = h.sccCredentials.Refresh()
	regCode, regErr := suseconnect.FetchSccRegistrationCodeFrom(h.secrets, activation.Status.RegistrationCodeSecretRef)
	if regErr != nil {
		return activation, regErr
	}

	sccConnection := suseconnect.DefaultRancherConnection(h.sccCredentials.SccCredentials())

	// TODO: remove override value - it's really just for testing
	identifier, version, arch := util.GetProductIdentifier("2.10.3")
	metaData, product, err := sccConnection.Activate(identifier, version, arch, regCode)
	if err != nil {
		return activation, err
	}
	logrus.Info(metaData)
	logrus.Info(product)

	status, statusErr := sccConnection.StatusPing(h.systemInfo)
	if statusErr != nil {
		return activation, statusErr
	}

	if status == sccRegistration.Registered {
		now := time.Now()
		logrus.Info("[scc.activation-controller]: Successfully registered activation")
		updated := activation.DeepCopy()
		updated.Status.LastValidatedTS = now.UTC().Format(time.RFC3339)
		updated.Status.ValidUntilTS = now.Add(24 * time.Hour).UTC().Format(time.RFC3339)
		updated.Status.Valid = true
		updated.Spec = v1.ActivationSpec{}
		return h.activations.UpdateStatus(updated)
	}

	return activation, nil
}

func (h *handler) processOfflineActivation(registration *v1.Activation) (*v1.Activation, error) {
	return registration, nil
}
