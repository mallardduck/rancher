package controllers

import (
	"fmt"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	regCode          = "regcode"
	registrationType = "registrationType"

	LabelSccLastProcessed = "scc.cattle.io/last-processsed"
	LabelSccHash          = "scc.cattle.io/scc-hash"
)

func extraRegistrationParamsFromSecret(secret *corev1.Secret) (RegistrationParams, error) {
	id, ok := secret.Labels[LabelSccLastProcessed]
	if !ok || len(id) == 0 {
		return RegistrationParams{}, fmt.Errorf("secret does not have label %s", LabelSccLastProcessed)
	}

	regCode, ok := secret.Data[regCode]
	if !ok || len(regCode) == 0 {
		return RegistrationParams{}, fmt.Errorf("secret does not have data %s", regCode)
	}

	regType, ok := secret.Labels[registrationType]
	if !ok || len(regType) == 0 {
		return RegistrationParams{}, fmt.Errorf("secret does not have label %s", registrationType)
	}

	return RegistrationParams{
		id:      id,
		regCode: string(regCode),
		regType: v1.RegistrationMode(regType),
	}, nil
}

type RegistrationParams struct {
	id      string
	regCode string
	regType v1.RegistrationMode
}

func (r RegistrationParams) Labels() map[string]string {
	return map[string]string{
		LabelSccHash: r.id,
	}
}

func registrationFromSecretEntrypoint(
	params RegistrationParams,
) (*v1.Registration, error) {
	if params.regType != v1.RegistrationModeOnline && params.regType != v1.RegistrationModeOffline {
		return nil, fmt.Errorf(
			"invalid registration type %s, must be one of %s or %s",
			params.regType,
			v1.RegistrationModeOnline,
			v1.RegistrationModeOffline,
		)
	}

	registration := &v1.Registration{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "registration-",
			Labels: map[string]string{
				LabelSccHash: fmt.Sprintf("%x", params.id),
			},
		},
		Spec: v1.RegistrationSpec{
			Mode:                params.regType,
			RegistrationRequest: &v1.RegistrationRequest{
				// TODO
			},
			OfflineRegistrationCertificateSecretRef: nil,
			SyncNow:                                 nil,
		},
	}

	return registration, nil
}
