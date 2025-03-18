package util

import (
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	RegCodeInitializerKey = "regCodeRef"
	RegCodeSecretName     = "rancher-scc-registration-code"
	RegCodeSecretKey      = "regCode"
	RegCertInitializerKey = "certificateRef"
	RegCertSecretName     = "rancher-scc-registration-certificate"
	RegCertSecretKey      = "certificate"
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
