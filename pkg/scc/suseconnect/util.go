package suseconnect

import (
	"errors"
	"fmt"
	"github.com/SUSE/connect-ng/pkg/connection"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/util"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FetchSccRegistrationCodeFrom(secrets controllerv1.SecretController, reference *corev1.SecretReference) (string, error) {
	regSecret, err := secrets.Get(reference.Namespace, reference.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	regCode, ok := regSecret.Data[util.RegCodeSecretKey]
	if !ok {
		return "", errors.New(fmt.Sprintf("registration secret `%v` does not contain expected data `%s`", reference, util.RegCodeSecretKey))
	}

	return string(regCode), nil
}

func StoreSccCredentials(secrets controllerv1.SecretController, credentials connection.Credentials) (*corev1.Secret, error) {
	token, tokenErr := credentials.Token()
	if tokenErr != nil {
		return nil, tokenErr
	}

	systemLogin, password, loginErr := credentials.Login()
	if loginErr != nil {
		return nil, loginErr
	}

	var newSecret *corev1.Secret
	secret, err := secrets.Get("cattle-system", util.RancherSCCSystemCredentialsSecretName, metav1.GetOptions{})
	if err == nil {
		secret.StringData = map[string]string{
			"systemToken": token,
			"systemLogin": systemLogin,
			"password":    password,
		}
		var err error
		newSecret, err = secrets.Update(secret)
		if err != nil {
			return nil, err
		}
	} else {
		tmp := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      util.RancherSCCSystemCredentialsSecretName,
				Namespace: "cattle-system",
			},
			StringData: map[string]string{
				"systemToken": token,
				"systemLogin": systemLogin,
				"password":    password,
			},
		}
		var err error
		newSecret, err = secrets.Create(tmp)
		if err != nil {
			return nil, err
		}
	}

	// TODO: update Request status to point to creds secret
	return newSecret, nil
}

func FetchSccCredentialsSecret(secrets controllerv1.SecretController) (*corev1.Secret, error) {
	secret, err := secrets.Get("cattle-system", util.RancherSCCSystemCredentialsSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func FetchSccCredentials(secrets controllerv1.SecretController) (connection.Credentials, error) {
	credentials := NewCredentials()
	secret, fetchErr := FetchSccCredentialsSecret(secrets)
	if fetchErr != nil {
		return nil, fetchErr
	}

	// Set credentials from secret data
	systemLogin := secret.Data["systemLogin"]
	systemPassword := secret.Data["password"]
	setErr := credentials.SetLogin(string(systemLogin), string(systemPassword))
	if setErr != nil {
		return nil, setErr
	}

	systemToken := secret.Data["systemToken"]
	tokenSetErr := credentials.UpdateToken(string(systemToken))
	if tokenSetErr != nil {
		return nil, tokenSetErr
	}

	return credentials, nil
}

func StoreSccOfflineRegistration(secrets controllerv1.SecretController, request *v1.Registration, offlineBlob []byte) (*corev1.Secret, error) {
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.RancherSCCOfflineRequestSecretName,
			Namespace: "cattle-system",
			Annotations: map[string]string{
				"owner": request.Name,
			},
		},
		StringData: map[string]string{
			util.RegCertSecretKey: string(offlineBlob),
		},
	}
	created, err := secrets.Create(newSecret)
	if err != nil {
		return nil, err
	}

	// TODO: update Request status to point to creds secret
	return created, nil
}
