package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
)

// RegistrationMode enforces the valid registration modes
// +kubebuilder:validation:Enum=online;offline
type RegistrationMode string

func (rm *RegistrationMode) Valid() bool {
	return *rm == Online || *rm == Offline
}

const (
	Online  RegistrationMode = "online"
	Offline RegistrationMode = "offline"
)

const (
	ResourceConditionDone        condition.Cond = "Done"
	ResourceConditionFailure     condition.Cond = "Failure"
	ResourceConditionProgressing condition.Cond = "Progressing"
	ResourceConditionReady       condition.Cond = "Ready"

	RegistrationConditionOfflineRequestReady     condition.Cond = "OfflineRequestReady"
	RegistrationConditionOfflineCertificateReady condition.Cond = "OfflineCertificateReady"
	ActivationConditionOfflineDone               condition.Cond = "OfflineActivationDone"

	RegistrationConditionAnnounced      condition.Cond = "RegistrationAnnounced"
	RegistrationConditionSccUrlReady    condition.Cond = "RegistrationSccUrlReady"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Registration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistrationSpec   `json:"spec,omitempty"`
	Status RegistrationStatus `json:"status,omitempty"`
}

type RegistrationSpec struct {
	// +default:value="online"
	Mode                                    RegistrationMode        `json:"mode"` // Either offline or online
	RegistrationCodeSecretRef               *corev1.SecretReference `json:"registrationCodeSecretRef,omitempty"`
	OfflineRegistrationCertificateSecretRef *corev1.SecretReference `json:"offlineRegistrationCertificateSecretRef,omitempty"`
	CheckNow                                bool                    `json:"checkNow,omitempty"` // for forcing Activation re-sync (for online mode) via k8s native methods
}

func (rs *RegistrationSpec) WithoutCheckNow() *RegistrationSpec {
	return &RegistrationSpec{
		Mode:                                    rs.Mode,
		RegistrationCodeSecretRef:               rs.RegistrationCodeSecretRef,
		OfflineRegistrationCertificateSecretRef: rs.OfflineRegistrationCertificateSecretRef,
	}
}

type RegistrationStatus struct {
	Conditions                 []genericcondition.GenericCondition `json:"conditions,omitempty"`
	SubscriptionInfo           SubscriptionInfo                    `json:"subscriptionInfo,omitempty"`
	RegistrationStatus         SystemRegistrationState             `json:"registrationStatus,omitempty"`
	ActivationStatus           SystemActivationState               `json:"activationStatus,omitempty"`
	SystemCredentialsSecretRef *corev1.SecretReference             `json:"systemCredentialsSecretRef,omitempty"`
	OfflineRegistrationRequest *corev1.SecretReference             `json:"offlineRegistrationRequest,omitempty"`
}

type SystemRegistrationState struct {
	SCCSystemId        int    `json:"sccSystemId,omitempty"`
	RequestProcessedTS string `json:"requestProcessedTS,omitempty"`
}

type SystemActivationState struct {
	Valid           bool   `json:"valid"`
	LastValidatedTS string `json:"lastValidatedTS"`
	ValidUntilTS    string `json:"validUntilTS"`
	Certificate     string `json:"certificate,omitempty"`
}
