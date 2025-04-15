package suseconnect

import (
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/pkg/errors"

	"github.com/SUSE/connect-ng/pkg/connection"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/rancher/rancher/pkg/scc/util"
)

type SccWrapper struct {
	credentials connection.Credentials
	conn        *connection.ApiConnection
	registered  bool
	systemInfo  *util.RancherSystemInfo
}

func DefaultConnectionOptions() connection.Options {
	// TODO: I believe this is creating the options for the API "app"
	// So this doesn't necessarily mean these have to match Rancher on the cluster.
	// Rather the details about the HTTP client talking to SCC
	return connection.DefaultOptions("rancher-scc-integration", "0.0.1", "en_US")
}

func DefaultRancherConnection(credentials connection.Credentials, systemInfo *util.RancherSystemInfo) SccWrapper {
	if credentials == nil {
		panic("credentials must be set")
	}
	options := DefaultConnectionOptions()
	// TODO: options in CLI tool set the base URL, shouldn't we do that too? (for SMT/RMT support)

	registered := false
	if credentials.HasAuthentication() {
		registered = true
	}

	return SccWrapper{
		credentials: credentials,
		conn:        connection.New(options, credentials),
		registered:  registered,
		systemInfo:  systemInfo,
	}
}

func (sw *SccWrapper) SubscriptionInfo(regCode string) (v1.SubscriptionInfo, error) {
	var subinfoResponse v1.SubscriptionInfo
	var err error
	request, err := sw.conn.BuildRequest("GET", "/connect/subscriptions/info", nil)
	if err != nil {
		return v1.SubscriptionInfo{}, err
	}

	connection.AddRegcodeAuth(request, regCode)

	responseBytes, reqErr := sw.conn.Do(request)
	if reqErr != nil {
		return v1.SubscriptionInfo{}, reqErr
	}

	jsonErr := json.Unmarshal(responseBytes, &subinfoResponse)
	if jsonErr != nil {
		return v1.SubscriptionInfo{}, jsonErr
	}

	return subinfoResponse, nil
}

type RegistrationSystemId int

// Define constant values for empty and error
const (
	EmptyRegistrationSystemId     RegistrationSystemId = 0  // Used if an error happened before registration
	ErrorRegistrationSystemId     RegistrationSystemId = -1 // Used when error is related to registration
	KeepAliveRegistrationSystemId RegistrationSystemId = -2 // Indicates the Registration was handled via keepalive instead
)

func (sw *SccWrapper) SystemRegistration(regCode string) (RegistrationSystemId, error) {
	// 1 collect system info
	systemInfo := sw.systemInfo
	preparedSystemInfo, err := systemInfo.PreparedForSCC()
	if err != nil {
		return EmptyRegistrationSystemId, err
	}

	id, regErr := registration.Register(sw.conn, regCode, systemInfo.ServerUrl(), preparedSystemInfo)
	if regErr != nil {
		return ErrorRegistrationSystemId, errors.Wrap(regErr, "Cannot register system to SCC")
	}

	return RegistrationSystemId(id), nil
}

func (sw *SccWrapper) KeepAlive() error {
	// 1 collect system info
	systemInfo := sw.systemInfo
	preparedSystemInfo, err := systemInfo.PreparedForSCC()
	if err != nil {
		return err
	}
	// 2 call Status
	status, statusErr := registration.Status(sw.conn, systemInfo.ServerUrl(), preparedSystemInfo)
	if status != registration.Registered {
		return fmt.Errorf("trying to send keepalive on a system that is not yet registered. register this system first: %v", statusErr)
	}
	// 3 verify response says we're registered still
	return statusErr
}

func (sw *SccWrapper) RegisterOrKeepAlive(regCode string) (RegistrationSystemId, error) {
	if sw.registered {
		return KeepAliveRegistrationSystemId, sw.KeepAlive()
	}

	return sw.SystemRegistration(regCode)
}

func (sw *SccWrapper) Activate(identifier string, version string, arch string, regCode string) (*registration.Metadata, *registration.Product, error) {
	metaData, product, err := registration.Activate(sw.conn, identifier, version, arch, regCode)
	if err != nil {
		return nil, nil, err
	}

	return metaData, product, err
}
