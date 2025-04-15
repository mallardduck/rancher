package util

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/rancher/rancher/pkg/wrangler"
	"net/url"

	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/rancher/rancher/pkg/settings"
	coreVersion "github.com/rancher/rancher/pkg/version"
)

type RancherSystemInfo struct {
	RancherUuid uuid.UUID
	ClusterUuid uuid.UUID
	Version     string
	wContext    *wrangler.Context
}

func NewRancherSystemInfo(rancherUuid uuid.UUID, clusterUuid uuid.UUID, wcontext *wrangler.Context) *RancherSystemInfo {
	var version string
	version = coreVersion.Version
	if version == "dev" {
		// TODO: maybe SCC devs can give us a static dev version?
		version = "2.10.3"
	}
	return &RancherSystemInfo{
		RancherUuid: rancherUuid,
		ClusterUuid: clusterUuid,
		Version:     version,
		wContext:    wcontext,
	}
}

func (rsi *RancherSystemInfo) ServerUrl() string {
	serverUrl := settings.ServerURL.Get()
	if serverUrl == "" {
		return ""
	}
	parsed, _ := url.Parse(serverUrl)
	return parsed.Host
}

func (rsi *RancherSystemInfo) preparedForSCC() ([]byte, error) {
	type RancherSCCInfo struct {
		UUID     uuid.UUID `json:"uuid"`
		Url      string    `json:"server_url"`
		Nodes    int       `json:"nodes"`
		Sockets  int       `json:"sockets"`
		Vcpus    int       `json:"vcpus"`
		Clusters int       `json:"clusters"`
		Version  string    `json:"version"`
	}

	// Fetch current node count
	clusterCount := rsi.wContext.Mgmt.Cluster().List()

	sccInfo := &RancherSCCInfo{
		UUID:     rsi.RancherUuid,
		Url:      rsi.ServerUrl(),
		Version:  rsi.Version,
		Nodes:    1,
		Sockets:  1,
		Vcpus:    1,
		Clusters: 0,
	}

	return json.Marshal(sccInfo)
}

func (rsi *RancherSystemInfo) PreparedForSCC() (registration.SystemInformation, error) {
	systemInfoMap := make(registration.SystemInformation)
	jsonInfo, err := rsi.preparedForSCC()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonInfo, &systemInfoMap)
	if err != nil {
		return nil, err
	}

	return systemInfoMap, nil
}

func (rsi *RancherSystemInfo) PreparedForSCCOffline() ([]byte, error) {
	return rsi.preparedForSCC()
}
