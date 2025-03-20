package util

import (
	"fmt"
	"github.com/SUSE/connect-ng/pkg/connection"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RegCodeInitializerKey                 = "regCodeRef"
	RegCodeSecretName                     = "rancher-scc-registration-code"
	RegCodeSecretKey                      = "regCode"
	RegCertInitializerKey                 = "certificateRef"
	RegCertSecretName                     = "rancher-scc-registration-certificate"
	RegCertSecretKey                      = "certificate"
	RancherSCCSystemCredentialsSecretName = "rancher-scc-system-credentials"
)

func ValidateInitializingConfigMap(sccInitializerConfig *corev1.ConfigMap) (string, *v1.RegistrationMode, error) {
	// Verify the expected fields are on the config map
	modeString, _ := sccInitializerConfig.Data["mode"]
	mode := v1.RegistrationMode(modeString)
	if !mode.Valid() {
		errorMsg := fmt.Sprintf("the configmap does not have a valid mode set")
		logrus.Error(errorMsg)
		return "", nil, fmt.Errorf(errorMsg)
	}

	credentialValueKey := ""
	if mode == v1.Online {
		credentialValueKey = RegCodeInitializerKey
	} else {
		credentialValueKey = RegCertInitializerKey
	}

	secretName, credOk := sccInitializerConfig.Data[credentialValueKey]
	if !credOk {
		// TODO bail here if OK is bad
		// Just unclear if we should: a) error, or b) silent error (letting `SCCFirstStart` get updated).
		errorMsg := fmt.Sprintf("cannot find the credential value key %s", credentialValueKey)
		logrus.Error(errorMsg)
		return "", nil, fmt.Errorf(errorMsg)
	}

	return secretName, &mode, nil
}

func StoreSccCredentials(secrets controllerv1.SecretController, request *v1.RegistrationRequest, credentials connection.Credentials) (*corev1.Secret, error) {
	token, tokenErr := credentials.Token()
	if tokenErr != nil {
		return nil, tokenErr
	}

	systemLogin, password, loginErr := credentials.Login()
	if loginErr != nil {
		return nil, loginErr
	}

	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RancherSCCSystemCredentialsSecretName,
			Namespace: "cattle-system",
		},
		StringData: map[string]string{
			"systemToken": token,
			"systemLogin": systemLogin,
			"password":    password,
		},
	}
	created, err := secrets.Create(newSecret)
	if err != nil {
		return nil, err
	}

	// TODO: update Request status to point to creds secret
	return created, nil
}

func RegistrationFromRequest(registrations registrationControllers.RegistrationController, request *v1.RegistrationRequest, secret *corev1.Secret) (*v1.Registration, error) {
	newRegistration := &v1.Registration{
		ObjectMeta: metav1.ObjectMeta{
			Name: request.Name,
		},
		Spec: v1.RegistrationSpec{},
		Status: v1.RegistrationStatus{
			Mode: request.Spec.Mode,
			OriginRegistrationRequestRef: &corev1.LocalObjectReference{
				Name: request.Name,
			},
			SystemCredentialsSecretRef: &corev1.SecretReference{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			},
		},
	}

	return registrations.Create(newRegistration)
}
