package util

import (
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/version"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RegCodeInitializerNameKey             = "regCodeSecretName"
	RegCodeInitializerNamespaceKey        = "regCodeSecretNamespace"
	RegCodeSecretName                     = "rancher-scc-registration-code"
	RegCodeSecretKey                      = "regCode"
	RegCertInitializerNameKey             = "certificateSecretName"
	RegCertInitializerNamespaceKey        = "certificateSecretNamespace"
	RegCertSecretName                     = "rancher-scc-registration-certificate"
	RegCertSecretKey                      = "certificate"
	RancherSCCSystemCredentialsSecretName = "rancher-scc-system-credentials"
	RancherSCCOfflineRequestSecretName    = "rancher-scc-offline-registration-request"
)

func ValidateInitializingConfigMap(sccInitializerConfig *corev1.ConfigMap) (*corev1.SecretReference, *v1.RegistrationMode, error) {
	secretReference := &corev1.SecretReference{}
	// Verify the expected fields are on the config map
	modeString, _ := sccInitializerConfig.Data["mode"]
	mode := v1.RegistrationMode(modeString)
	if !mode.Valid() {
		errorMsg := fmt.Sprintf("the configmap does not have a valid mode set")
		logrus.Error(errorMsg)
		return secretReference, nil, fmt.Errorf(errorMsg)
	}

	credentialNameKey := ""
	credentialNamespaceKey := ""
	if mode == v1.Online {
		credentialNameKey = RegCodeInitializerNameKey
		credentialNamespaceKey = RegCodeInitializerNamespaceKey
	} else {
		credentialNameKey = RegCertInitializerNameKey
		credentialNamespaceKey = RegCertInitializerNamespaceKey
	}

	secretName, credOk := sccInitializerConfig.Data[credentialNameKey]
	if !credOk {
		// TODO bail here if OK is bad
		// Just unclear if we should: a) error, or b) silent error (letting `SCCFirstStart` get updated).
		errorMsg := fmt.Sprintf("cannot find the credential value key %s", credentialNameKey)
		logrus.Error(errorMsg)
		return secretReference, nil, fmt.Errorf(errorMsg)
	}

	secretNamespace, credOk := sccInitializerConfig.Data[credentialNamespaceKey]
	if !credOk {
		// TODO bail here if OK is bad
		// Just unclear if we should: a) error, or b) silent error (letting `SCCFirstStart` get updated).
		errorMsg := fmt.Sprintf("cannot find the credential value key %s", credentialNamespaceKey)
		logrus.Error(errorMsg)
		secretNamespace = "cattle-system"
	}

	secretReference.Name = secretName
	secretReference.Namespace = secretNamespace

	return secretReference, &mode, nil
}

func RegistrationFromRequest(registrations registrationControllers.RegistrationController, request *v1.RegistrationRequest) (*v1.Registration, error) {
	existingReg, err := registrations.Get(request.Name, metav1.GetOptions{})
	if err != nil {
		return existingReg, nil
	}

	newRegStatus := v1.RegistrationStatus{
		Mode: request.Spec.Mode,
		OriginRegistrationRequestRef: &corev1.LocalObjectReference{
			Name: request.Name,
		},
		RegistrationCodeSecretRef:  request.Spec.RegistrationCodeSecretRef.DeepCopy(),
		SystemCredentialsSecretRef: request.Status.SystemCredentialsSecretRef.DeepCopy(),
	}
	newRegistration := &v1.Registration{
		ObjectMeta: metav1.ObjectMeta{
			Name: request.Name,
		},
		Spec:   v1.RegistrationSpec{},
		Status: newRegStatus,
	}

	newRegistration, err = registrations.Create(newRegistration)
	if err != nil {
		return &v1.Registration{}, err
	}

	newRegistration = newRegistration.DeepCopy()
	newRegistration.Status = newRegStatus

	return registrations.UpdateStatus(newRegistration)
}

func GetProductIdentifier(override string) (string, string, string) {
	if override != "" {
		return "rancher", override, "unknown"
	}

	return "rancher", version.Version, "unknown"
}
