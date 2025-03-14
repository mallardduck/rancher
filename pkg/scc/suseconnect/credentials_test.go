package suseconnect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredentialTypeStrings(t *testing.T) {
	assert.Equal(t, "unconfigured", CredentialTypeUnconfigured.String())
	assert.Equal(t, "token", CredentialTypeToken.String())
	assert.Equal(t, "login", CredentialTypeLogin.String())
	assert.Equal(t, "token and login", CredentialTypeBoth.String())
}

func TestNewCredentials(t *testing.T) {
	credential := NewCredentials("login", "password")
	assert.Equal(t, "login", credential.login)
	assert.Equal(t, "password", credential.password)
}

func TestSetLogin(t *testing.T) {
	credential := Credentials{
		login:    "",
		password: "",
	}
	err := credential.SetLogin("newLogin", "newPassword")
	assert.NoError(t, err)

	credential = Credentials{
		login:    "",
		password: "",
	}
	err = credential.SetLogin("", "newPassword")
	assert.Error(t, err)
}

func TestSetLoginEmptyFields(t *testing.T) {
	credential := Credentials{
		login:    "",
		password: "",
	}
	err := credential.SetLogin("", "")
	assert.Error(t, err)
}

func TestSetLoginMultipleErrors(t *testing.T) {
	credential := Credentials{
		login:    "",
		password: "",
	}
	err := credential.SetLogin("", "newPassword")
	assert.Error(t, err)

	err = credential.SetLogin("newLogin", "")
	assert.Error(t, err)
}

func TestHasAuthentication(t *testing.T) {
	credential := Credentials{
		token:    "token",
		login:    "login",
		password: "password",
	}
	assert.True(t, credential.HasAuthentication())

	credential = Credentials{
		token:    "",
		login:    "login",
		password: "password",
	}
	assert.True(t, credential.HasAuthentication())

	credential = Credentials{
		token:    "",
		login:    "",
		password: "",
	}
	assert.False(t, credential.HasAuthentication())
}

func TestToken(t *testing.T) {
	credential := Credentials{
		token:    "token",
		login:    "",
		password: "",
	}
	token, err := credential.Token()
	assert.Equal(t, "token", token)
	assert.NoError(t, err)
	err = credential.UpdateToken("newTokenTest")
	assert.NoError(t, err)

	token, err = credential.Token()
	assert.NotEqual(t, "token", token)
	assert.Equal(t, "newTokenTest", token)
	assert.NoError(t, err)
}

func TestTokenErrors(t *testing.T) {
	credential := Credentials{
		token:    "",
		login:    "",
		password: "",
	}
	_, err := credential.Token()
	assert.Error(t, err)

	err = credential.UpdateToken("testToken")
	assert.NoError(t, err)
}

func TestUpdateTokenErrors(t *testing.T) {
	credential := Credentials{
		token:    "",
		login:    "",
		password: "",
	}
	err := credential.UpdateToken("")
	assert.Error(t, err)
}

func TestCredentialsType(t *testing.T) {
	credential := Credentials{
		token:    "",
		login:    "login",
		password: "password",
	}
	assert.Equal(t, CredentialTypeLogin, credential.CredentialsType())

	credential = Credentials{
		token:    "",
		login:    "login",
		password: "",
	}
	assert.Equal(t, CredentialTypeUnconfigured, credential.CredentialsType())

	credential = Credentials{
		token:    "token",
		login:    "",
		password: "",
	}
	assert.Equal(t, CredentialTypeToken, credential.CredentialsType())

	credential = Credentials{
		token:    "token",
		login:    "login",
		password: "",
	}
	assert.Equal(t, CredentialTypeToken, credential.CredentialsType())

	credential = Credentials{
		token:    "",
		login:    "",
		password: "",
	}
	assert.Equal(t, CredentialTypeUnconfigured, credential.CredentialsType())
}

func TestCredentialsEmpty(t *testing.T) {
	credential := NewCredentials("", "")
	assert.Equal(t, CredentialTypeUnconfigured, credential.CredentialsType())

	emptyCreds := Credentials{
		token:    "",
		login:    "",
		password: "",
	}
	assert.Equal(t, emptyCreds, credential)

}

func TestLogin(t *testing.T) {
	credential := Credentials{
		token:    "",
		login:    "user1",
		password: "pass1",
	}
	login, pass, err := credential.Login()
	assert.Equal(t, "user1", login)
	assert.Equal(t, "pass1", pass)
	assert.NoError(t, err)

	err = credential.SetLogin("user2", "pass2")
	assert.NoError(t, err)

	login, pass, err = credential.Login()
	assert.NotEqual(t, "user1", login)
	assert.Equal(t, "user2", login)
	assert.NotEqual(t, "pass1", pass)
	assert.Equal(t, "pass2", pass)
	assert.NoError(t, err)
}

func TestLoginErrors(t *testing.T) {
	credential := Credentials{
		token:    "",
		login:    "",
		password: "",
	}
	_, _, err := credential.Login()
	assert.Error(t, err)

	err = credential.SetLogin("user3", "")
	assert.Error(t, err)

	err = credential.SetLogin("", "pass3")
	assert.Error(t, err)

	err = credential.SetLogin("user3", "pass3")
	assert.NoError(t, err)
}
