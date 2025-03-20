package suseconnect

import (
	"github.com/SUSE/connect-ng/pkg/connection"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/pkg/errors"
)

func DefaultConnectionOptions() connection.Options {
	// TODO: I believe this is creating the options for the API "app"
	// So this doesn't necessarily mean these have to match Rancher on the cluster.
	// Rather the details about the HTTP client talking to SCC
	return connection.DefaultOptions("rancher-scc-integration", "0.0.1", "en_US")
}

func DefaultRancherConnection(credentials connection.Credentials) *connection.ApiConnection {
	options := DefaultConnectionOptions()
	if credentials == nil {
		credentials = connection.NoCredentials{}
	}
	return connection.New(options, credentials)
}

func SubscriptionInfo(conn *connection.ApiConnection, regCode string) ([]byte, error) {
	var subinfoResponse []byte
	var err error
	request, err := conn.BuildRequest("GET", "/connect/subscriptions/info", nil)
	if err != nil {
		return subinfoResponse, err
	}

	connection.AddRegcodeAuth(request, regCode)

	subinfoResponse, err = conn.Do(request)
	if err != nil {
		return subinfoResponse, err
	}

	return subinfoResponse, nil
}

func SystemRegistration(conn *connection.ApiConnection, regCode string, hostname string, systemInformation any) (int, error) {
	id, regErr := registration.Register(conn, regCode, hostname, systemInformation)
	if regErr != nil {
		return 0, errors.Wrap(regErr, "Cannot register system to SCC")
	}

	return id, nil
}
