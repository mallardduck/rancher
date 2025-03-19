package suseconnect

import (
	"github.com/SUSE/connect-ng/pkg/connection"
)

func DefaultConnectionOptions() connection.Options {
	// TODO: I believe this is creating the options for the API "app"
	// So this doesn't necessarily mean these have to match Rancher on the cluster.
	// Rather the details about the HTTP client talking to SCC
	return connection.DefaultOptions("rancher-scc-integration", "0.0.1", "en_US")
}

func DefaultRancherConnection() *connection.ApiConnection {
	options := DefaultConnectionOptions()
	return connection.New(options, connection.NoCredentials{})
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
