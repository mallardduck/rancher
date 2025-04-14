package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"

	"github.com/sirupsen/logrus"
)

type handler struct {
	ctx            context.Context
	registrations  registrationControllers.RegistrationController
	activations    registrationControllers.ActivationController
	configMaps     v1core.ConfigMapController
	secrets        v1core.SecretController
	sccCredentials *credentials.CredentialSecretsAdapter
	systemInfo     *util.RancherSystemInfo
}

func Register(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
	activations registrationControllers.ActivationController,
	configMaps v1core.ConfigMapController,
	secrets v1core.SecretController,
	systemInfo *util.RancherSystemInfo,
) {
	controller := &handler{
		ctx:            ctx,
		registrations:  registrations,
		activations:    activations,
		configMaps:     configMaps,
		secrets:        secrets,
		sccCredentials: credentials.New(secrets),
		systemInfo:     systemInfo,
	}

	registrations.OnChange(ctx, "registrations", controller.OnRegistrationChange)
}

func (h *handler) OnRegistrationChange(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, nil
	}

	logrus.Infof("[scc.registration-controller]: Received registration %q", name)
	logrus.Info("[scc.registration-controller]: Registration ", registrationObj)
	// 1. Verify Registration is not already fulfilled or expired,
	// TODO: implement expiration - gist: Registration shouldn't repeat to infinity when issues.
	// Ideally we would eventually timeout a Registration after it fails X times or for X minutes
	if registrationObj.Status.RequestProcessedTS != "" {
		logrus.Info("[scc.registration-controller]: Registration already processed")
		return registrationObj, nil
	}

	if h.systemInfo.ServerUrl() == "" {
		logrus.Info("[scc.registration-controller]: Server URL not set")
		return registrationObj, errors.New("no server url found in the system info")
	}

	// TODO: set a status so we know this is currently processing
	var err error
	// 2. Verify contents of Registration (mode and creds),
	if registrationObj.Spec.Mode == v1.Online {
		registrationObj, err = h.processOnlineRegistration(registrationObj)
		if err != nil {
			return h.setReconcilingCondition(registrationObj, err)
		}
	} else {
		// TODO: potentially this should be based on other state?
		if registrationObj.Status.OfflineRegistrationRequest == nil {
			registrationObj, err = h.prepareOfflineRegistrationRequest(registrationObj)
			if err != nil {
				return h.setReconcilingCondition(registrationObj, err)
			}
		} else if registrationObj.Spec.RegistrationCertificateSecretRef != nil {
			registrationObj, err = h.processOfflineRegistration(registrationObj)
			if err != nil {
				return h.setReconcilingCondition(registrationObj, err)
			}
		}
	}

	// 4. At the end of either process the current Registration is either:
	// 		a) fulfilled, b) expired or c) failed (retry?)
	v1.ResourceConditionDone.SetStatusBool(registrationObj, true)
	v1.ResourceConditionSynced.SetStatusBool(registrationObj, true)
	registrationObj, err = h.registrations.UpdateStatus(registrationObj)
	if err != nil {
		return h.setReconcilingCondition(registrationObj, err)
	}

	// 4+. If it was a success, then a new Registration is created (and the old one deleted or marked as not current?)
	return registrationObj, nil
}

func (h *handler) processOnlineRegistration(registrationObj *v1.Registration) (*v1.Registration, error) {
	_ = h.sccCredentials.Refresh()
	logrus.Info("[scc.registration-controller]: online mode ")

	v1.ResourceConditionProgressing.SetStatusBool(registrationObj, true)
	registrationObj, err := h.registrations.UpdateStatus(registrationObj)
	if err != nil {
		return registrationObj, err
	}

	// 1. Verify the secret ref name
	secretRef := &corev1.SecretReference{
		Namespace: "cattle-system",
		Name:      util.RegCodeSecretName,
	}
	if registrationObj.Spec.RegistrationCodeSecretRef != nil {
		secretRef = registrationObj.Spec.RegistrationCodeSecretRef
	}

	// 2. Verify Registration creds,
	registrationCode, regErr := suseconnect.FetchSccRegistrationCodeFrom(h.secrets, secretRef)
	if regErr != nil {
		return registrationObj, regErr
	}

	serverUrl := h.systemInfo.ServerUrl()
	if serverUrl == "" {
		return h.setReconcilingCondition(registrationObj, errors.New("no server url found in systemInfo"))
	}

	// 2. Attempt SCC phone home with Online mode
	sccConnection := suseconnect.DefaultRancherConnection(h.sccCredentials.SccCredentials())
	registrationObj, err = h.verifyBasicSubscription(registrationObj, &sccConnection, registrationCode)
	if err != nil {
		return h.setReconcilingCondition(registrationObj, err)
	}

	registrationObj, err = h.createSystemRegistration(registrationObj, &sccConnection, registrationCode)
	if err != nil {
		return h.setReconcilingCondition(registrationObj, err)
	}

	// TODO: Create the registration CRD
	_, activationErr := util.ActivationFromRegistration(h.activations, registrationObj)
	if activationErr != nil {
		// TODO: consider conditions when this would fail, some may need IgnoreErr to prevent useless retries
		return registrationObj, activationErr
	}

	// TODO: If we reached here we can set a success condition!
	registrationObj.Status.RequestProcessedTS = time.Now().UTC().Format(time.RFC3339)
	registrationObj.Status.Conditions = make([]genericcondition.GenericCondition, 0)
	v1.ResourceConditionFailure.SetStatusBool(registrationObj, false)
	v1.ResourceConditionReady.SetStatusBool(registrationObj, true)
	registrationObj, err = h.registrations.UpdateStatus(registrationObj)
	if err != nil {
		return registrationObj, err
	}

	return registrationObj, nil
}

func (h *handler) verifyBasicSubscription(registrationObj *v1.Registration, sccConnection *suseconnect.SccWrapper, regCode string) (*v1.Registration, error) {
	subscriptionInfoResponse, err := sccConnection.SubscriptionInfo(regCode)
	if err != nil {
		return registrationObj, err
	}

	newRegRequest := registrationObj.DeepCopy()
	newRegRequest.Status.SubscriptionInfo = string(subscriptionInfoResponse)

	subscriptionInfo := registration.SubscriptionInfo{}
	err = json.Unmarshal(subscriptionInfoResponse, &subscriptionInfo)
	if err != nil {
		return registrationObj, err
	}

	if !util.ValidateRancherProductClass(subscriptionInfo.ProductClasses) {
		regError := errors.New("the provided Registration Code doesn't match Rancher product class")
		v1.RegistrationConditionInvalidProduct.SetStatusBool(newRegRequest, true)
		v1.RegistrationConditionInvalidProduct.SetError(newRegRequest, "", regError)

		newRegRequest, newErr := h.registrations.UpdateStatus(newRegRequest)
		if newErr != nil {
			return newRegRequest, errors.Wrap(newErr, regError.Error())
		}
		return newRegRequest, nil
	}

	now := time.Now()
	if subscriptionInfo.StartsAt.After(now) || subscriptionInfo.ExpiresAt.Before(now) {
		return registrationObj, errors.New(fmt.Sprintf("subscription info is out of date"))
	}

	return h.registrations.UpdateStatus(newRegRequest)
}

func (h *handler) createSystemRegistration(registrationObj *v1.Registration, sccConnection *suseconnect.SccWrapper, code string) (*v1.Registration, error) {
	// If this step has been done don't repeat it
	if registrationObj.Status.SubscriptionInfo != "" && registrationObj.Status.SCCSystemId != 0 && registrationObj.Status.SystemCredentialsSecretRef != nil {
		// TODO: this may need more verification
		return registrationObj, nil
	}

	hostname := h.systemInfo.ServerUrl()
	systemInfo, err := h.systemInfo.PreparedForSCC()
	if err != nil {
		return registrationObj, err
	}

	id, regErr := sccConnection.SystemRegistration(code, hostname, systemInfo)
	if regErr != nil {
		return registrationObj, regErr
	}
	logrus.Infof("!! check https://scc.suse.com/systems/%d\n", id)

	newRegRequest := registrationObj.DeepCopy()
	newRegRequest.Status.SCCSystemId = id
	newRegRequest.Status.SystemCredentialsSecretRef = &corev1.SecretReference{
		Namespace: credentials.Namespace,
		Name:      credentials.SecretName,
	}
	// TODO add a status condition for this too...
	// Lets set the link as the message for that status too: https://scc.suse.com/systems/%d
	newRegRequest, err = h.registrations.UpdateStatus(newRegRequest)
	if err != nil {
		return registrationObj, err
	}

	return newRegRequest, nil
}

func (h *handler) prepareOfflineRegistrationRequest(registrationObj *v1.Registration) (*v1.Registration, error) {
	logrus.Info("[scc.registration-controller]: offline mode create request")
	sccOfflineBlob, jsonErr := h.systemInfo.PreparedForSCCOffline()
	if jsonErr != nil {
		return registrationObj, jsonErr
	}
	offlineRegistrationSecret, err := suseconnect.StoreSccOfflineRegistration(h.secrets, registrationObj, sccOfflineBlob)
	if err != nil {
		return registrationObj, err
	}

	updatedRequest := registrationObj.DeepCopy()
	updatedRequest.Status.OfflineRegistrationRequest = &corev1.SecretReference{
		Name:      offlineRegistrationSecret.Name,
		Namespace: offlineRegistrationSecret.Namespace,
	}

	// TODO: also set a status/condition to indicate Offline is ready for user
	// The message could potentially even give command to fetch secret?

	updatedRequest, err = h.registrations.UpdateStatus(updatedRequest)
	if err != nil {
		return registrationObj, err
	}

	return updatedRequest, nil
}

func (h *handler) processOfflineRegistration(registrationObj *v1.Registration) (*v1.Registration, error) {
	// TODO implement offline mechanism
	logrus.Info("[scc.registration-controller]: offline mode processing")
	return registrationObj, nil
}

func (h *handler) setReconcilingCondition(request *v1.Registration, originalErr error) (*v1.Registration, error) {
	logrus.Info("[scc.registration-controller]: set reconciling condition")
	logrus.Error(originalErr)

	// TODO Update status
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		updBackup, err := h.registrations.Get(request.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		updBackup = updBackup.DeepCopy()
		v1.ResourceConditionFailure.SetStatusBool(updBackup, true)
		v1.ResourceConditionFailure.SetError(updBackup, "", originalErr)
		v1.ResourceConditionReady.Message(updBackup, "Retrying")
		v1.ResourceConditionProgressing.SetStatusBool(updBackup, false)

		_, err = h.registrations.UpdateStatus(updBackup)
		return err
	})
	if err != nil {
		return request, errors.New(originalErr.Error() + err.Error())
	}

	return request, originalErr
}
