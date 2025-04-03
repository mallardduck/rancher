package util

import (
	"encoding/json"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/google/uuid"
	"github.com/rancher/rancher/pkg/settings"
	"net/url"
)

type RancherSystemInfo struct {
	RancherUuid uuid.UUID
	ClusterUuid uuid.UUID
	// TODO: these count based items may make more sense as getters on `RancherSystemInfo`
	Nodes    int
	Sockets  int
	Vcpus    int
	Clusters int
	Version  string
}

func (rsi *RancherSystemInfo) ServerUrl() string {
	serverUrl := settings.ServerURL.Get()
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

	sccInfo := &RancherSCCInfo{
		UUID:     rsi.RancherUuid,
		Url:      rsi.ServerUrl(),
		Nodes:    rsi.Nodes,
		Sockets:  rsi.Sockets,
		Vcpus:    rsi.Vcpus,
		Clusters: rsi.Clusters,
		//Version:  rsi.Version,
		Version: "2.10.3",
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
