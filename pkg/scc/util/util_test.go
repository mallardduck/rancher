package util

import (
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValidateRancherProductClass(t *testing.T) {
	exampleProductClasses := []registration.ProductClass{
		{"RANCHER-X86", ""},
	}
	valid := ValidateRancherProductClass(exampleProductClasses)
	assert.True(t, valid)

	exampleProductClasses = []registration.ProductClass{
		{"OBSERVABILITY-X86", ""},
	}
	invalid := ValidateRancherProductClass(exampleProductClasses)
	assert.False(t, invalid)

	exampleProductClasses = []registration.ProductClass{
		{"OBSERVABILITY-X86", ""},
		{"RANCHER-X86", ""},
	}
	valid = ValidateRancherProductClass(exampleProductClasses)
	assert.True(t, valid)
}
