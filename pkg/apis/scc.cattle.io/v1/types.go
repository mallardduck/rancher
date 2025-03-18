package v1

import (
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RegistrationRequestConditionUnprocessed condition.Cond = "Unprocessed"
	RegistrationRequestConditionReady       condition.Cond = "Ready"
	RegistrationRequestConditionFailed      condition.Cond = "Failed"
	RegistrationConditionHealthy            condition.Cond = "Healthy"
	RegistrationConditionPending            condition.Cond = "Pending"
	RegistrationConditionExpired            condition.Cond = "Expired"
	RegistrationConditionCloned             condition.Cond = "Cloned"
	RegistrationConditionTimeout            condition.Cond = "Timeout"
	RegistrationConditionFailed             condition.Cond = "Failed"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RegistrationRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistrationRequestSpec   `json:"spec,omitempty"`
	Status RegistrationRequestStatus `json:"status,omitempty"`
}

type RegistrationRequestSpec struct {
	// +default:value="online"
	Mode                             RegistrationMode        `json:"mode"` // Either offline or online
	RegistrationCodeSecretRef        *corev1.SecretReference `json:"registrationCodeSecretRef,omitempty"`
	RegistrationCertificateSecretRef *corev1.SecretReference `json:"registrationCertificateSecretRef,omitempty"`
}

type RegistrationRequestStatus struct {
	Conditions                 []genericcondition.GenericCondition `json:"conditions,omitempty"`
	RequestProcessedTS         string                              `json:"requestProcessedTS"`
	OfflineRegistrationRequest *corev1.SecretReference             `json:"offlineRegistrationRequest,omitempty"`
}

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

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Registration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RegistrationSpec   `json:"spec,omitempty"`
	Status            RegistrationStatus `json:"status,omitempty"`
}

type RegistrationSpec struct {
	CheckNow bool `json:"checkNow,omitempty"`
}

type RegistrationStatus struct {
	Mode            RegistrationMode                    `json:"mode"`
	Valid           bool                                `json:"valid"`
	LastValidatedTS string                              `json:"lastValidatedTS"`
	ValidUntilTS    string                              `json:"validUntilTS"`
	Certificate     string                              `json:"certificate"`
	Conditions      []genericcondition.GenericCondition `json:"conditions,omitempty"`
}
