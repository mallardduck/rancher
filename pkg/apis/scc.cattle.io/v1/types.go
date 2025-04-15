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
	ResourceConditionSynced      condition.Cond = "Synced"

	RegistrationConditionInvalidProduct condition.Cond = "InvalidProduct"
	RegistrationConditionSystemUrlReady condition.Cond = "SystemUrlReady"
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
	Mode                             RegistrationMode        `json:"mode"` // Either offline or online
	RegistrationCodeSecretRef        *corev1.SecretReference `json:"registrationCodeSecretRef,omitempty"`
	RegistrationCertificateSecretRef *corev1.SecretReference `json:"registrationCertificateSecretRef,omitempty"`
}

type RegistrationStatus struct {
	Conditions                 []genericcondition.GenericCondition `json:"conditions,omitempty"`
	SubscriptionInfo           SubscriptionInfo                    `json:"subscriptionInfo,omitempty"`
	SCCSystemId                int                                 `json:"sccSystemId,omitempty"`
	SystemCredentialsSecretRef *corev1.SecretReference             `json:"systemCredentialsSecretRef,omitempty"`
	RequestProcessedTS         string                              `json:"requestProcessedTS,omitempty"`
	OfflineRegistrationRequest *corev1.SecretReference             `json:"offlineRegistrationRequest,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Activation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ActivationSpec   `json:"spec,omitempty"`
	Status            ActivationStatus `json:"status,omitempty"`
}

type ActivationSpec struct {
	CheckNow bool `json:"checkNow,omitempty"`
}

type ActivationStatus struct {
	Mode                       RegistrationMode                    `json:"mode"`
	OriginRegistrationRef      *corev1.LocalObjectReference        `json:"originRegistration,omitempty"`
	RegistrationCodeSecretRef  *corev1.SecretReference             `json:"registrationCodeSecretRef,omitempty"`
	SystemCredentialsSecretRef *corev1.SecretReference             `json:"systemCredentialsSecretRef,omitempty"`
	Valid                      bool                                `json:"valid"`
	LastValidatedTS            string                              `json:"lastValidatedTS"`
	ValidUntilTS               string                              `json:"validUntilTS"`
	Certificate                string                              `json:"certificate,omitempty"`
	Conditions                 []genericcondition.GenericCondition `json:"conditions,omitempty"`
}
