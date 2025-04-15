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

type ProductClass struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type SubscriptionInfo struct {
	Name           string         `yaml:"name,omitempty" json:"name,omitempty"`
	StartsAt       metav1.Time    `yaml:"startsAt,omitempty" json:"starts_at,omitempty"`
	ExpiresAt      metav1.Time    `yaml:"expiresAt,omitempty" json:"expires_at,omitempty"`
	ProductClasses []ProductClass `yaml:"productClass,omitempty" json:"product_classes,omitempty"`
}

const (
	ResourceConditionDone        condition.Cond = "Done"
	ResourceConditionFailure     condition.Cond = "Failure"
	ResourceConditionProgressing condition.Cond = "Progressing"
	ResourceConditionReady       condition.Cond = "Ready"

	RegistrationConditionAnnounced      condition.Cond = "RegistrationAnnounced"
	RegistrationConditionInvalidProduct condition.Cond = "RegistrationInvalidProduct"
	RegistrationConditionSystemUrlReady condition.Cond = "RegistrationSystemUrlReady"
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
