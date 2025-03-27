package util

import (
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCombinedUUID(t *testing.T) {
	uuid1, _ := uuid.Parse("bea71e55-a1ec-4e5f-a5c0-c0e10b1a571c")
	uuid2, _ := uuid.Parse("d4e1f8c0-9f47-47e8-8c11-8fa97ff29ba6")

	combined := combinedUUID(uuid1, uuid2)
	const expected = "748a66f5-eef1-55cb-82db-4ef39b9d20ab"
	assert.Equal(t, expected, combined.String())
	expectedUUID, _ := uuid.Parse(expected)
	assert.Equal(t, expectedUUID, combined)
}

func TestDuplicateCombinedUUID(t *testing.T) {
	uuid1, _ := uuid.Parse("bea71e55-a1ec-4e5f-a5c0-c0e10b1a571c")

	combined := combinedUUID(uuid1, uuid1)
	const expected = "babb64f9-8b86-5f0b-bfdd-a654314e3788"
	assert.Equal(t, expected, combined.String())
	expectedUUID, _ := uuid.Parse(expected)
	assert.Equal(t, expectedUUID, combined)
}

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
