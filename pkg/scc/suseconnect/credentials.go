package suseconnect

import (
	"fmt"
	"github.com/pkg/errors"
)

type CredentialType int

const (
	CredentialTypeUnconfigured CredentialType = iota
	CredentialTypeToken
	CredentialTypeLogin
	CredentialTypeBoth
)

var credentialTypeName = map[CredentialType]string{
	CredentialTypeUnconfigured: "unconfigured",
	CredentialTypeToken:        "token",
	CredentialTypeLogin:        "login",
	CredentialTypeBoth:         "token and login",
}

func (ct CredentialType) String() string {
	return credentialTypeName[ct]
}

type SccCredentials struct {
	token    string
	login    string
	password string
}

// CredentialsType Returns the mode (or modes) that are configured for authentication
func (c *SccCredentials) CredentialsType() CredentialType {
	if c.token != "" && c.login != "" && c.password != "" {
		return CredentialTypeBoth
	}

	if c.token != "" {
		return CredentialTypeToken
	}

	if c.login != "" && c.password != "" {
		return CredentialTypeLogin
	}

	return CredentialTypeUnconfigured
}

// HasAuthentication Returns true if we can authenticate at all, false otherwise.
func (c *SccCredentials) HasAuthentication() bool {
	configuredType := c.CredentialsType()
	if configuredType != CredentialTypeUnconfigured {
		return true
	}

	return false
}

// Token returns the current system used to detect duplicated systems. This
// token gets rotated on each non read operation.
func (c *SccCredentials) Token() (string, error) {
	if c.token == "" {
		return "", errors.New("the token is not currently set")
	}

	return c.token, nil
}

// UpdateToken is called when a token has changed
func (c *SccCredentials) UpdateToken(newToken string) error {
	if newToken == "" {
		return errors.New("cannot update token to empty string")
	}

	c.token = newToken

	return nil
}

// Login returns the username and password
func (c *SccCredentials) Login() (string, string, error) {
	configuredType := c.CredentialsType()
	if configuredType != CredentialTypeLogin && configuredType != CredentialTypeBoth {
		return "", "", errors.New("cannot use login credentials when they are not properly configured")
	}

	return c.login, c.password, nil
}

// SetLogin updates the saved username and password
func (c *SccCredentials) SetLogin(newLogin string, newPassword string) error {
	if newLogin == "" || newPassword == "" {
		errorMessage := ""
		if newLogin == "" {
			errorMessage += "newLogin is empty"
		}
		if newPassword == "" {
			if newLogin == "" {
				errorMessage += " and "
			}
			errorMessage += "newPassword is empty"
		}
		return errors.New(fmt.Sprintf("cannot update login; %v", errorMessage))
	}

	c.login = newLogin
	c.password = newPassword

	return nil
}

func NewCredentials(login string, password string) SccCredentials {
	credential := SccCredentials{
		login:    login,
		password: password,
	}
	return credential
}
