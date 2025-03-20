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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

func Register(
	ctx context.Context,
	registrationRequests registrationControllers.RegistrationRequestController,
	registrations registrationControllers.RegistrationController,
	configMaps v1core.ConfigMapController,
	secrets v1core.SecretController,
) {
	controller := &handler{
		ctx:                  ctx,
		registrationRequests: registrationRequests,
		registrations:        registrations,
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
	secretName := util.RegCodeSecretName
	if registrationRequest.Spec.RegistrationCodeSecretRef != nil {
		secretName = registrationRequest.Spec.RegistrationCodeSecretRef.Name
	}

	// 2. Verify RegistrationRequest creds,
	regSecret, err := h.secrets.Get("cattle-system", secretName, metav1.GetOptions{})
	if err != nil {
		return registrationRequest, err
	}

	regCode, ok := regSecret.Data[util.RegCodeSecretKey]
	if !ok {
		return registrationRequest, errors.New(fmt.Sprintf("registration secret `%s` does not contain expected data `%s`", secretName, util.RegCodeSecretKey))
	}

	// 2. Attempt SCC phone home with Online mode
	sccCredentials := suseconnect.SccCredentials{}
	sccConnection := suseconnect.DefaultRancherConnection(&sccCredentials)
	registrationCode := string(regCode)
	registrationRequest, err = h.verifyBasicSubscription(registrationRequest, sccConnection, registrationCode)
	if err != nil {
		return h.setReconcilingCondition(registrationRequest, err)
	}

	registrationRequest, err = h.createSystemRegistration(registrationRequest, sccConnection, registrationCode)
	if err != nil {
		return h.setReconcilingCondition(registrationRequest, err)
	}

	// TODO: If we reached here we can set a success condition!

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

	// TODO: this should check status/condition to verify it needs to be done.
	// If we have valid system credentials we don't need to repeat this I think?

	// TODO: get hostname of cluster - also needs to fail until we know the hostname (domain of cluster)
	hostname := "dpock-test.not-real-hostname.lan"

	id, regErr := suseconnect.SystemRegistration(sccConnection, code, hostname, nil)
	if regErr != nil {
		return registrationRequest, regErr
	}
	logrus.Infof("!! check https://scc.suse.com/systems/%d\n", id)

	newRegRequest := registrationRequest.DeepCopy()
	newRegRequest.Status.SCCSystemId = id
	// TODO add a status condition for this too...
	// Lets set the link as the message for that status too: https://scc.suse.com/systems/%d
	newRegRequest, err := h.registrationRequests.UpdateStatus(newRegRequest)
	if err != nil {
		return registrationRequest, err
	}

	// Ensure we keep the system credentials we were just issued
	credentialsSecret, credsErr := util.StoreSccCredentials(h.secrets, registrationRequest, sccConnection.GetCredentials())
	if credsErr != nil {
		return registrationRequest, credsErr
	}

	logrus.Info(credentialsSecret)
	_, registrationErr := util.RegistrationFromRequest(h.registrations, registrationRequest, credentialsSecret)
	if registrationErr != nil {
		// TODO: consider conditions when this would fail, some may need IgnoreErr to prevent useless retries
		return registrationRequest, registrationErr
	}

	// Setting the RequestProcessedTS will ensure this doesn't get reprocessed again
	newRegRequest2 := newRegRequest.DeepCopy()
	newRegRequest2.Status.RequestProcessedTS = time.Now().String()
	newRegRequest2, err = h.registrationRequests.UpdateStatus(newRegRequest2)
	if err != nil {
		return newRegRequest, err
	}

	return newRegRequest2, nil
}

func (h *handler) prepareOfflineRegistrationRequest(registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	// TODO implement offline mechanism
	logrus.Info("[scc.registrationrequest-controller]: offline mode ")
	return registrationRequest, nil
}

func (h *handler) processOfflineRegistration(registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	// TODO implement offline mechanism
	logrus.Info("[scc.registrationrequest-controller]: offline mode ")
	return registrationRequest, nil
}

func (h *handler) setReconcilingCondition(request *v1.RegistrationRequest, originalErr error) (*v1.RegistrationRequest, error) {
	logrus.Info("[scc.registrationrequest-controller]: set reconciling condition")
	logrus.Error(originalErr)

	// TODO Update status

	return request, originalErr
}
