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
	registrationRequest, err := h.setProcessingCondition(registrationRequest)
	if err != nil {
		return nil, errors.New("[scc.registrationrequest-controller]: setting condition failed;" + err.Error())
	}

	// 2. Verify contents of RegistrationRequest (mode and creds),
	if registrationRequest.Spec.Mode == v1.Online {
		registrationRequest, err := h.processOnlineRegistration(registrationRequest)
		if err != nil {
			return h.setReconcilingCondition(registrationRequest, err)
		}
	} else {
		err := h.processOfflineRegistration(registrationRequest)
		if err != nil {
			return h.setReconcilingCondition(registrationRequest, err)
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
		return &v1.RegistrationRequest{}, err
	}

	regCode, ok := regSecret.Data[util.RegCodeSecretKey]
	if !ok {
		return &v1.RegistrationRequest{}, errors.New(fmt.Sprintf("registration secret `%s` does not contain expected data `%s`", secretName, util.RegCodeSecretKey))
	}

	// 2. Attempt SCC phone home with Online mode
	sccConnection := suseconnect.DefaultRancherConnection()
	registrationCode := string(regCode)
	registrationRequest, err = h.verifyBasicSubscription(registrationRequest, sccConnection, registrationCode)
	if err != nil {
		return &v1.RegistrationRequest{}, err
	}

	registrationRequest, err = h.setSuccessCondition(registrationRequest)
	if err != nil {
		return &v1.RegistrationRequest{}, err
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

	v1.RegistrationRequestConditionSubscriptionInfoCollected.SetStatusBool(newRegRequest, true)
	registrationRequest, err = h.registrationRequests.UpdateStatus(newRegRequest)
	if err != nil {
		return registrationRequest, err
	}

	return newRegRequest, nil
}

func (h *handler) setProcessingCondition(registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	v1.RegistrationRequestConditionProcessing.SetStatusBool(registrationRequest, true)
	v1.RegistrationRequestConditionProcessing.SetMessageIfBlank(registrationRequest, "SCC RegistrationRequest Processing")

	return h.registrationRequests.UpdateStatus(registrationRequest)
}

func (h *handler) processOfflineRegistration(registrationRequest *v1.RegistrationRequest) error {
	// TODO implement offline mechanism
	logrus.Info("[scc.registrationrequest-controller]: offline mode ")
	return nil
}

func (h *handler) setReconcilingCondition(request *v1.RegistrationRequest, originalErr error) (*v1.RegistrationRequest, error) {
	logrus.Info("[scc.registrationrequest-controller]: set reconciling condition")
	logrus.Error(originalErr)

	// TODO implement backoff in here?
	err := h.setFailedCondition(request, originalErr)
	if err != nil {
		return request, errors.New(originalErr.Error() + err.Error())
	}

	return request, originalErr
}

// TODO: use this when we implement retry interval and timeout fully
// TODO: pass error to this and set the message
func (h *handler) setBackoffCondition(registrationRequest *v1.RegistrationRequest) error {
	v1.RegistrationRequestConditionProcessing.SetStatusBool(registrationRequest, false)
	v1.RegistrationRequestConditionProcessing.SetMessageIfBlank(registrationRequest, "SCC RegistrationRequest Completed")

	v1.RegistrationRequestConditionBackoff.SetStatusBool(registrationRequest, true)
	v1.RegistrationRequestConditionBackoff.SetMessageIfBlank(registrationRequest, "Processing failed for now, will retry soon.")

	registrationRequest, err := h.registrationRequests.UpdateStatus(registrationRequest)
	return err
}

// TODO: pass error to this and set the message
func (h *handler) setFailedCondition(registrationRequest *v1.RegistrationRequest, originalError error) error {
	v1.RegistrationRequestConditionProcessing.SetStatusBool(registrationRequest, false)
	v1.RegistrationRequestConditionCompleted.SetStatusBool(registrationRequest, false)
	v1.RegistrationRequestConditionCompleted.SetMessageIfBlank(registrationRequest, "Failed to process RegistrationRequest")

	// Failed communicates that it won't be retried, and error communicates the logged error
	// TODO: actually set the message to something that makes sense based on the error
	v1.RegistrationRequestConditionFailed.SetStatusBool(registrationRequest, true)
	v1.RegistrationRequestConditionFailed.SetError(registrationRequest, "", originalError)

	registrationRequest, err := h.registrationRequests.UpdateStatus(registrationRequest)
	return errors.New(originalError.Error() + err.Error())
}

func (h *handler) setSuccessCondition(registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	v1.RegistrationRequestConditionProcessing.SetStatusBool(registrationRequest, false)
	v1.RegistrationRequestConditionProcessing.SetMessageIfBlank(registrationRequest, "SCC RegistrationRequest Completed")

	v1.RegistrationRequestConditionCompleted.SetStatusBool(registrationRequest, true)
	v1.RegistrationRequestConditionCompleted.SetMessageIfBlank(registrationRequest, "Success")

	if v1.RegistrationConditionFailed.GetStatus(registrationRequest) != "" {
		v1.RegistrationRequestConditionFailed.SetStatusBool(registrationRequest, false)
	}

	return h.registrationRequests.UpdateStatus(registrationRequest)
}
