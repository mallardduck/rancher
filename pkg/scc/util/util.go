package util

import (
	"encoding/json"
	"fmt"
	"github.com/SUSE/connect-ng/pkg/connection"
	"github.com/google/uuid"
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
	RancherSCCOfflineRequestSecretName    = "rancher-scc-offline-registration-request"
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

type RancherSystemInfo struct {
	ClusterUuid uuid.UUID
	RancherUuid uuid.UUID
	Url         string
	Nodes       int
	Sockets     int
	Vcpus       int
	Clusters    int
	Version     string
}

func (rsi *RancherSystemInfo) Uuid() uuid.UUID {
	return combinedUUID(rsi.ClusterUuid, rsi.RancherUuid)
}

func (rsi *RancherSystemInfo) PreparedForSCC() ([]byte, error) {
	type RancherSCCInfo struct {
		UUID     uuid.UUID `json:"uuid"`
		Url      string    `json:"server_url"`
		Nodes    int       `json:"nodes"`
		Sockets  int       `json:"sockets"`
		Vcpus    int       `json:"vcpus"`
		Clusters int       `json:"clusters"`
		Version  string    `json:"version"`
	}

	sccInfo := &RancherSCCInfo{
		UUID:     rsi.Uuid(),
		Url:      rsi.Url,
		Nodes:    rsi.Nodes,
		Sockets:  rsi.Sockets,
		Vcpus:    rsi.Vcpus,
		Clusters: rsi.Clusters,
		//Version:  rsi.Version,
		Version: "2.10.3",
	}

	return json.Marshal(sccInfo)
}

func combinedUUID(uuid1, uuid2 uuid.UUID) uuid.UUID {
	// Combine the byte representations of the two UUIDs.
	combinedBytes := append(uuid1[:], uuid2[:]...)

	// Use uuid.NewSHA1 to generate the combined UUID.
	return uuid.NewSHA1(uuid1, combinedBytes)
}

func StoreSccOfflineRegistration(secrets controllerv1.SecretController, request *v1.RegistrationRequest, offlineBlob []byte) (*corev1.Secret, error) {
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RancherSCCOfflineRequestSecretName,
			Namespace: "cattle-system",
			Annotations: map[string]string{
				"owner": request.Name,
			},
		},
		StringData: map[string]string{
			"offlineRequest": string(offlineBlob),
		},
	}
	created, err := secrets.Create(newSecret)
	if err != nil {
		return nil, err
	}

	// TODO: update Request status to point to creds secret
	return created, nil
}
