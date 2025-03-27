package suseconnect

import (
	"github.com/SUSE/connect-ng/pkg/connection"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/scc/util"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
)

type SccWrapper struct {
	secrets v1core.SecretController
	conn    *connection.ApiConnection
}

func DefaultConnectionOptions() connection.Options {
	// TODO: I believe this is creating the options for the API "app"
	// So this doesn't necessarily mean these have to match Rancher on the cluster.
	// Rather the details about the HTTP client talking to SCC
	return connection.DefaultOptions("rancher-scc-integration", "0.0.1", "en_US")
}

func DefaultRancherConnection(secrets v1core.SecretController, credentials connection.Credentials) SccWrapper {
	options := DefaultConnectionOptions()
	if credentials == nil {
		credentials = connection.NoCredentials{}
	}
	return SccWrapper{
		secrets: secrets,
		conn:    connection.New(options, credentials),
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

func (sw *SccWrapper) SystemRegistration(regCode string, hostname string, systemInformation any) (int, *v1.Secret, error) {
	id, regErr := registration.Register(sw.conn, regCode, hostname, systemInformation)
	if regErr != nil {
		return 0, nil, errors.Wrap(regErr, "Cannot register system to SCC")
	}

	// TODO: this may need to be done in errors and in success
	credentialsSecret, credsErr := StoreSccCredentials(sw.secrets, sw.conn.GetCredentials())
	if credsErr != nil {
		return 0, credentialsSecret, credsErr
	}

	return id, credentialsSecret, nil
}

func (sw *SccWrapper) Activate(identifier string, version string, arch string, regCode string) (*registration.Metadata, *registration.Product, error) {
	metaData, product, err := registration.Activate(sw.conn, identifier, version, arch, regCode)
	if err != nil {
		return nil, nil, err
	}

	// TODO: this may need to be done in errors and in success
	_, credsErr := StoreSccCredentials(sw.secrets, sw.conn.GetCredentials())
	if credsErr != nil {
		return nil, nil, credsErr
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

	// TODO: this may need to be done in errors and in success
	_, credsErr := StoreSccCredentials(sw.secrets, sw.conn.GetCredentials())
	if credsErr != nil {
		return status, credsErr
	}

	return status, nil
}
