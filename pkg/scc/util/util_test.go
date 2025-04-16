package util

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValidateRancherProductClass(t *testing.T) {
	exampleProductClasses := []v1.ProductClass{
		{"RANCHER-X86", ""},
	}
	valid := ValidateRancherProductClass(exampleProductClasses)
	assert.True(t, valid)

	exampleProductClasses = []v1.ProductClass{
		{"OBSERVABILITY-X86", ""},
	}
	invalid := ValidateRancherProductClass(exampleProductClasses)
	assert.False(t, invalid)

	exampleProductClasses = []v1.ProductClass{
		{"OBSERVABILITY-X86", ""},
		{"RANCHER-X86", ""},
	}
	valid = ValidateRancherProductClass(exampleProductClasses)
	assert.True(t, valid)
}
