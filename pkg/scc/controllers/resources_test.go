package controllers

import (
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestRegistrationResourceCreation(t *testing.T) {
	type testcase struct {
		secret      *corev1.Secret
		expected    *v1.Registration
		paramsErr   error
		generateErr error
	}

	assert := assert.New(t)

	tcs := []testcase{
		{
			secret:      nil,
			expected:    nil,
			paramsErr:   nil,
			generateErr: nil,
		},
	}

	for _, tc := range tcs {
		params, err := extraRegistrationParamsFromSecret(tc.secret)
		if tc.paramsErr != nil {
			assert.ErrorIs(err, tc.paramsErr)
		} else {
			assert.NoError(err)
		}

		registration, err := registrationFromSecretEntrypoint(params)
		if tc.generateErr != nil {
			assert.ErrorIs(err, tc.generateErr)
		} else {
			assert.NoError(err)
			assert.Equal(tc.expected, registration)
		}
	}
}
