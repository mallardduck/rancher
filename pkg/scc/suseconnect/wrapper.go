package suseconnect

import (
	"github.com/SUSE/connect-ng/pkg/connection"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/scc/util"
)

type SccWrapper struct {
	credentials connection.Credentials
	conn        *connection.ApiConnection
}

func DefaultConnectionOptions() connection.Options {
	// TODO: I believe this is creating the options for the API "app"
	// So this doesn't necessarily mean these have to match Rancher on the cluster.
	// Rather the details about the HTTP client talking to SCC
	return connection.DefaultOptions("rancher-scc-integration", "0.0.1", "en_US")
}

func DefaultRancherConnection(credentials connection.Credentials) SccWrapper {
	// TODO: don't panic on this, handle it better
	if credentials == nil {
		panic("credentials must be set")
	}
	options := DefaultConnectionOptions()
	return SccWrapper{
		credentials: credentials,
		conn:        connection.New(options, credentials),
	}
}

func (sw *SccWrapper) SubscriptionInfo(regCode string) ([]byte, error) {
	var subinfoResponse []byte
	var err error
	request, err := sw.conn.BuildRequest("GET", "/connect/subscriptions/info", nil)
	if err != nil {
		return subinfoResponse, err
	}

	connection.AddRegcodeAuth(request, regCode)

	subinfoResponse, err = sw.conn.Do(request)
	if err != nil {
		return subinfoResponse, err
	}

	// TODO: this may need us to update the credentials - though we may not have them yet?

	return subinfoResponse, nil
}

func (sw *SccWrapper) SystemRegistration(regCode string, hostname string, systemInformation any) (int, error) {
	id, regErr := registration.Register(sw.conn, regCode, hostname, systemInformation)
	if regErr != nil {
		return 0, errors.Wrap(regErr, "Cannot register system to SCC")
	}

	return id, nil
}

func (sw *SccWrapper) Activate(identifier string, version string, arch string, regCode string) (*registration.Metadata, *registration.Product, error) {
	metaData, product, err := registration.Activate(sw.conn, identifier, version, arch, regCode)
	if err != nil {
		return nil, nil, err
	}

	return metaData, product, err
}

func (sw *SccWrapper) StatusPing(systemInfo *util.RancherSystemInfo) (registration.StatusCode, error) {
	preparedSystemInfo, err := systemInfo.PreparedForSCC()
	if err != nil {
		return registration.Unknown, err
	}

	status, statusErr := registration.Status(sw.conn, systemInfo.ServerUrl(), preparedSystemInfo)
	if statusErr != nil {
		return status, statusErr
	}

	return status, nil
}
