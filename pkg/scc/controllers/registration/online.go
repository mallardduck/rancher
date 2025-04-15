package registration

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type onlineHandler struct {
	rootHandler *handler
}

func (oh *onlineHandler) Run(registrationObj *v1.Registration) (*v1.Registration, error) {
	// This is an extra verification borne of paranoia - technically the controller won't start till this is ready.
	// However, people can do silly things so if someone unsets the server URL this will block that.
	if !oh.rootHandler.isServerUrlReady() {
		return registrationObj, errors.New("cannot process registration if `server-url` is not configured")
	}

	if v1.ResourceConditionDone.IsTrue(registrationObj) && v1.RegistrationConditionAnnounced.IsTrue(registrationObj) {
		logrus.Debugf("[scc.registration-controller]: registration already complete, nothing to process for %s", registrationObj.Name)
		return registrationObj, nil
	}

	logrus.Debug("[scc.registration-controller]: online mode registration starting")
	credErr := oh.rootHandler.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credErr != nil {
		return registrationObj, fmt.Errorf("cannot refresh credentials: %w", credErr)
	}

	v1.ResourceConditionProgressing.SetStatusBool(registrationObj, true)
	registrationObj, err := oh.rootHandler.registrations.UpdateStatus(registrationObj)
	if err != nil {
		return registrationObj, err
	}

	// Set to global default, or user configured value from the Registration resource
	regCodeSecretRef := &corev1.SecretReference{
		Namespace: "cattle-system",
		Name:      util.RegCodeSecretName,
	}
	if registrationObj.Spec.RegistrationCodeSecretRef != nil {
		regObjRegCodeSecretRef := registrationObj.Spec.RegistrationCodeSecretRef
		if regObjRegCodeSecretRef.Name != "" && regObjRegCodeSecretRef.Namespace != "" {
			regCodeSecretRef = registrationObj.Spec.RegistrationCodeSecretRef
		} else {
			logrus.Warn("[scc.registration-controller]: registration code secret reference was set but cannot be used")
		}
	}

	// Verify the SCC regcode secret exists
	registrationCode, regErr := suseconnect.FetchSccRegistrationCodeFrom(oh.rootHandler.secrets, regCodeSecretRef)
	if regErr != nil {
		return registrationObj, regErr
	}

	// Initiate connection to SCC & verify reg code is for Rancher
	sccConnection := suseconnect.DefaultRancherConnection(oh.rootHandler.sccCredentials.SccCredentials(), oh.rootHandler.systemInfo)
	registrationObj, err = oh.verifyBasicSubscription(registrationObj, &sccConnection, registrationCode)
	if err != nil {
		return registrationObj, err
	}

	// Announce this Rancher cluster to SCC
	registrationObj, err = oh.announceSystem(registrationObj, &sccConnection, registrationCode)
	if err != nil {
		return registrationObj, err
	}

	// Prepare an Activation from the Registration
	_, activationErr := util.ActivationFromRegistration(oh.rootHandler.activations, registrationObj)
	if activationErr != nil {
		// TODO: consider conditions when this would fail, some may need IgnoreErr to prevent useless retries
		return registrationObj, activationErr
	}

	completeObj := registrationObj.DeepCopy()
	completeObj.Status.RegistrationStatus.RequestProcessedTS = time.Now().UTC().Format(time.RFC3339)
	completeObj.Status.Conditions = make([]genericcondition.GenericCondition, 0)
	v1.ResourceConditionFailure.SetStatusBool(completeObj, false)
	v1.ResourceConditionReady.SetStatusBool(completeObj, true)
	completeObj, finalUpdateErr := oh.rootHandler.registrations.UpdateStatus(completeObj)
	if finalUpdateErr != nil {
		return registrationObj, finalUpdateErr
	}

	return completeObj, nil
}

func (oh *onlineHandler) verifyBasicSubscription(registrationObj *v1.Registration, sccConnection *suseconnect.SccWrapper, regCode string) (*v1.Registration, error) {
	subscriptionInfoResponse, err := sccConnection.SubscriptionInfo(regCode)
	if err != nil {
		return registrationObj, err
	}

	newRegRequest := registrationObj.DeepCopy()
	newRegRequest.Status.SubscriptionInfo = subscriptionInfoResponse

	if !util.ValidateRancherProductClass(subscriptionInfoResponse.ProductClasses) {
		regError := errors.New("the provided Registration Code doesn't match Rancher product class")
		v1.RegistrationConditionInvalidProduct.SetStatusBool(newRegRequest, true)
		v1.RegistrationConditionInvalidProduct.SetError(newRegRequest, "", regError)

		var newErr error
		newRegRequest, newErr = oh.rootHandler.registrations.UpdateStatus(newRegRequest)
		if newErr != nil {
			return registrationObj, errors.Wrap(newErr, regError.Error())
		}
		return newRegRequest, nil
	}

	now := time.Now()
	startsAt := newRegRequest.Status.SubscriptionInfo.StartsAt.Time
	expiresAt := newRegRequest.Status.SubscriptionInfo.ExpiresAt.Time
	if now.Before(startsAt) || now.After(expiresAt) {
		return registrationObj, errors.New(fmt.Sprintf("subscription info is out of date"))
	}

	return oh.rootHandler.registrations.UpdateStatus(newRegRequest)
}

func (oh *onlineHandler) announceSystem(registrationObj *v1.Registration, sccConnection *suseconnect.SccWrapper, code string) (*v1.Registration, error) {
	id, regErr := sccConnection.RegisterOrKeepAlive(code)
	if regErr != nil {
		// TODO, do we error different based on ID type?
		return registrationObj, regErr
	}

	if id == suseconnect.KeepAliveRegistrationSystemId {
		// TODO something to update status from keepalive
		return registrationObj, nil
	}

	sccSystemUrl := fmt.Sprintf("https://scc.suse.com/system/%d", id)
	logrus.Debugf("[scc.registration-controller]: system announced, check %s", sccSystemUrl)

	newRegObj := registrationObj.DeepCopy()
	v1.RegistrationConditionSystemUrlReady.SetStatusBool(newRegObj, false) // This must be false until successful activation too.
	v1.RegistrationConditionSystemUrlReady.SetMessageIfBlank(newRegObj, fmt.Sprintf("system announced, check %s", sccSystemUrl))

	newRegObj.Status.RegistrationStatus.SCCSystemId = int(id)
	newRegObj.Status.SystemCredentialsSecretRef = &corev1.SecretReference{
		Namespace: credentials.Namespace,
		Name:      credentials.SecretName,
	}

	var updateErr error
	newRegObj, updateErr = oh.rootHandler.registrations.UpdateStatus(newRegObj)
	if updateErr != nil {
		return registrationObj, updateErr
	}

	return newRegObj, nil
}
