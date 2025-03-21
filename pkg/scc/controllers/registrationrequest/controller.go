package registrationrequest

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SUSE/connect-ng/pkg/connection"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
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
	ctx                  context.Context
	registrationRequests registrationControllers.RegistrationRequestController
	registrations        registrationControllers.RegistrationController
	configMaps           v1core.ConfigMapController
	secrets              v1core.SecretController
	systemInfo           *util.RancherSystemInfo
}

func Register(
	ctx context.Context,
	registrationRequests registrationControllers.RegistrationRequestController,
	registrations registrationControllers.RegistrationController,
	configMaps v1core.ConfigMapController,
	secrets v1core.SecretController,
	systemInfo *util.RancherSystemInfo,
) {
	controller := &handler{
		ctx:                  ctx,
		registrationRequests: registrationRequests,
		registrations:        registrations,
		configMaps:           configMaps,
		secrets:              secrets,
		systemInfo:           systemInfo,
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
	// TODO: set a status so we know this is currently processing

	var err error
	// 2. Verify contents of RegistrationRequest (mode and creds),
	if registrationRequest.Spec.Mode == v1.Online {
		registrationRequest, err = h.processOnlineRegistration(registrationRequest)
		if err != nil {
			return h.setReconcilingCondition(registrationRequest, err)
		}
	} else {
		// TODO: potentially this should be based on other state?
		if registrationRequest.Status.OfflineRegistrationRequest == nil {
			registrationRequest, err = h.prepareOfflineRegistrationRequest(registrationRequest)
			if err != nil {
				return h.setReconcilingCondition(registrationRequest, err)
			}
		} else if registrationRequest.Spec.RegistrationCertificateSecretRef != nil {
			registrationRequest, err = h.processOfflineRegistration(registrationRequest)
			if err != nil {
				return h.setReconcilingCondition(registrationRequest, err)
			}
		}
	}

	// 4. At the end of either process the current RegistrationRequest is either:
	// 		a) fulfilled, b) expired or c) failed (retry?)
	// 4+. If it was a success, then a new Registration is created (and the old one deleted or marked as not current?)
	return registrationRequest, nil
}

func (h *handler) processOnlineRegistration(registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	logrus.Info("[scc.registrationrequest-controller]: online mode ")
	// 1. Verify the secret ref name
	secretRef := &corev1.SecretReference{
		Namespace: "cattle-system",
		Name:      util.RegCodeSecretName,
	}
	if registrationRequest.Spec.RegistrationCodeSecretRef != nil {
		secretRef = registrationRequest.Spec.RegistrationCodeSecretRef
	}

	// 2. Verify RegistrationRequest creds,
	registrationCode, regErr := suseconnect.FetchSccRegistrationCodeFrom(h.secrets, secretRef)
	if regErr != nil {
		return registrationRequest, regErr
	}

	serverUrl := h.systemInfo.ServerUrl()
	if serverUrl == "" {
		return h.setReconcilingCondition(registrationRequest, errors.New("no server url found in systemInfo"))
	}

	// 2. Attempt SCC phone home with Online mode
	sccCredentials := suseconnect.SccCredentials{}
	sccConnection := suseconnect.DefaultRancherConnection(&sccCredentials)
	registrationRequest, err := h.verifyBasicSubscription(registrationRequest, sccConnection, registrationCode)
	if err != nil {
		return h.setReconcilingCondition(registrationRequest, err)
	}

	registrationRequest, err = h.createSystemRegistration(registrationRequest, sccConnection, registrationCode)
	if err != nil {
		return h.setReconcilingCondition(registrationRequest, err)
	}

	// TODO: Create the registration CRD
	_, registrationErr := util.RegistrationFromRequest(h.registrations, registrationRequest)
	if registrationErr != nil {
		// TODO: consider conditions when this would fail, some may need IgnoreErr to prevent useless retries
		return registrationRequest, registrationErr
	}

	// TODO: If we reached here we can set a success condition!
	registrationRequest.Status.RequestProcessedTS = time.Now().UTC().Format(time.RFC3339)
	registrationRequest.Status.Conditions = make([]genericcondition.GenericCondition, 0)
	v1.ResourceConditionFailure.SetStatusBool(registrationRequest, false)
	v1.ResourceConditionReady.SetStatusBool(registrationRequest, true)
	registrationRequest, err = h.registrationRequests.UpdateStatus(registrationRequest)
	if err != nil {
		return registrationRequest, err
	}

	return registrationRequest, nil
}

func (h *handler) verifyBasicSubscription(registrationRequest *v1.RegistrationRequest, sccConnection *connection.ApiConnection, regCode string) (*v1.RegistrationRequest, error) {
	subscriptionInfoResponse, err := suseconnect.SubscriptionInfo(sccConnection, regCode)
	if err != nil {
		return registrationRequest, err
	}

	subscriptionInfo := registration.SubscriptionInfo{}
	err = json.Unmarshal(subscriptionInfoResponse, &subscriptionInfo)
	if err != nil {
		return registrationRequest, err
	}

	now := time.Now()
	if subscriptionInfo.StartsAt.After(now) || subscriptionInfo.ExpiresAt.Before(now) {
		return registrationRequest, errors.New(fmt.Sprintf("subscription info is out of date"))
	}

	newRegRequest := registrationRequest.DeepCopy()
	newRegRequest.Status.SubscriptionInfo = string(subscriptionInfoResponse)
	newRegRequest, err = h.registrationRequests.UpdateStatus(newRegRequest)
	if err != nil {
		return registrationRequest, err
	}

	return newRegRequest, nil
}

func (h *handler) createSystemRegistration(registrationRequest *v1.RegistrationRequest, sccConnection *connection.ApiConnection, code string) (*v1.RegistrationRequest, error) {
	// If this step has been done don't repeat it
	if registrationRequest.Status.SubscriptionInfo != "" && registrationRequest.Status.SCCSystemId != 0 && registrationRequest.Status.SystemCredentialsSecretRef != nil {
		// TODO: this may need more verification
		return registrationRequest, nil
	}

	hostname := h.systemInfo.ServerUrl()
	systemInfo, err := h.systemInfo.PreparedForSCC()
	if err != nil {
		return registrationRequest, err
	}

	id, regErr := suseconnect.SystemRegistration(sccConnection, code, hostname, systemInfo)
	if regErr != nil {
		return registrationRequest, regErr
	}
	logrus.Infof("!! check https://scc.suse.com/systems/%d\n", id)

	// Ensure we keep the system credentials we were just issued
	credentialsSecret, credsErr := suseconnect.StoreSccCredentials(h.secrets, sccConnection.GetCredentials())
	if credsErr != nil {
		return registrationRequest, credsErr
	}

	newRegRequest := registrationRequest.DeepCopy()
	newRegRequest.Status.SCCSystemId = id
	newRegRequest.Status.SystemCredentialsSecretRef = &corev1.SecretReference{
		Namespace: credentialsSecret.GetNamespace(),
		Name:      credentialsSecret.GetName(),
	}
	// TODO add a status condition for this too...
	// Lets set the link as the message for that status too: https://scc.suse.com/systems/%d
	newRegRequest, err = h.registrationRequests.UpdateStatus(newRegRequest)
	if err != nil {
		return registrationRequest, err
	}

	return newRegRequest, nil
}

func (h *handler) prepareOfflineRegistrationRequest(registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	logrus.Info("[scc.registrationrequest-controller]: offline mode create request")
	sccOfflineBlob, jsonErr := h.systemInfo.PreparedForSCCOffline()
	if jsonErr != nil {
		return registrationRequest, jsonErr
	}
	offlineRegistrationSecret, err := suseconnect.StoreSccOfflineRegistration(h.secrets, registrationRequest, sccOfflineBlob)
	if err != nil {
		return registrationRequest, err
	}

	updatedRequest := registrationRequest.DeepCopy()
	updatedRequest.Status.OfflineRegistrationRequest = &corev1.SecretReference{
		Name:      offlineRegistrationSecret.Name,
		Namespace: offlineRegistrationSecret.Namespace,
	}

	// TODO: also set a status/condition to indicate Offline is ready for user
	// The message could potentially even give command to fetch secret?

	updatedRequest, err = h.registrationRequests.UpdateStatus(updatedRequest)
	if err != nil {
		return registrationRequest, err
	}

	return updatedRequest, nil
}

func (h *handler) processOfflineRegistration(registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	// TODO implement offline mechanism
	logrus.Info("[scc.registrationrequest-controller]: offline mode processing")
	return registrationRequest, nil
}

func (h *handler) setReconcilingCondition(request *v1.RegistrationRequest, originalErr error) (*v1.RegistrationRequest, error) {
	logrus.Info("[scc.registrationrequest-controller]: set reconciling condition")
	logrus.Error(originalErr)

	// TODO Update status
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		updBackup, err := h.registrationRequests.Get(request.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		updBackup = updBackup.DeepCopy()
		v1.ResourceConditionFailure.SetStatusBool(updBackup, true)
		v1.ResourceConditionFailure.SetError(updBackup, "", originalErr)
		v1.ResourceConditionReady.Message(updBackup, "Retrying")
		v1.ResourceConditionProgressing.SetStatusBool(updBackup, false)

		_, err = h.registrationRequests.UpdateStatus(updBackup)
		return err
	})
	if err != nil {
		return request, errors.New(originalErr.Error() + err.Error())
	}

	return request, originalErr
}
