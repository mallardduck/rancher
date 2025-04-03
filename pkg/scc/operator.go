package scc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/version"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	sccv1 "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"

	"github.com/rancher/rancher/pkg/scc/controllers/activation"
	"github.com/rancher/rancher/pkg/scc/controllers/registration"
	"github.com/rancher/rancher/pkg/wrangler"
)

type sccOperator struct {
	registrations     sccv1.RegistrationController
	activations       sccv1.ActivationController
	configMaps        v1core.ConfigMapController
	secrets           v1core.SecretController
	systemInformation *util.RancherSystemInfo
}

func setup(wContext *wrangler.Context) (*sccOperator, error) {
	namespaces := wContext.Core.Namespace()
	kubeSystemNS, err := namespaces.Get("kube-system", metav1.GetOptions{})
	if err != nil {
		// fatal log here, because we need the kube-system ns UID while creating any backup file
		logrus.Fatalf("Error getting namespace kube-system %v", err)
	}

	rancherUuid := settings.InstallUUID.Get()
	if rancherUuid == "" {
		err := errors.New("no rancher uuid found")
		logrus.Fatalf("Error getting rancher uuid: %v", err)
		return nil, err
	}

	// TODO: also get Node, Sockets, Vcpus, Clusters and watch those
	return &sccOperator{
		registrations: wContext.SCC.Registration(),
		activations:   wContext.SCC.Activation(),
		configMaps:    wContext.Core.ConfigMap(),
		secrets:       wContext.Core.Secret(),
		systemInformation: &util.RancherSystemInfo{
			RancherUuid: uuid.MustParse(rancherUuid),
			ClusterUuid: uuid.MustParse(string(kubeSystemNS.UID)),
			Version:     version.Version,
		},
	}, nil
}

// maybeFirstInit will check if the initial `Registration` seeding values exist
// and if they need to be processed into a new `Registration` (used during first boot ever)
func (so *sccOperator) maybeFirstInit() (*v1.Registration, error) {
	logrus.Info("SCC controller MaybeFirstInit")
	if strings.EqualFold(settings.SCCFirstStart.Get(), "false") {
		logrus.Warn("Skipping the SCC controller first start; first start already completed previously.")
		return nil, nil
	}

	// Check if the `cattle-system:initial-scc-registration` ConfigMap exists
	// If it does not, then we simply proceed and mark the setting as false
	configMap, err := so.configMaps.Get("cattle-system", "initial-scc-registration", metav1.GetOptions{})
	var newRegistration *v1.Registration
	if err != nil {
		logrus.Warn("Cannot find initial-scc-registration configmap; it will be skipped")
	} else {
		secretRef, mode, err := util.ValidateInitializingConfigMap(configMap)
		if err != nil {
			logrus.Warn("Cannot validate initial-scc-registration configmap; it will be skipped")
		} else {
			newRegistration = &v1.Registration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "rancher-",
				},
			}
			newRegistration.Spec.Mode = *mode
			if *mode == v1.Offline {
				newRegistration.Spec.RegistrationCertificateSecretRef = secretRef
			} else {
				newRegistration.Spec.RegistrationCodeSecretRef = secretRef
			}

			_, err = so.registrations.Create(newRegistration)
			if err != nil {
				logrus.Errorf("Cannot create registration request; %s", err)
				return nil, err
			}
			_ = so.configMaps.Delete(configMap.Namespace, configMap.Name, &metav1.DeleteOptions{})
		}
	}

	// At very end, we will set it to false so this doesn't run again
	if !strings.EqualFold(settings.SCCFirstStart.Get(), "false") {
		if err := settings.SCCFirstStart.Set("false"); err != nil {
			return newRegistration, err
		}
	}

	return newRegistration, nil
}

func Setup(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	logrus.Info("Starting SCC Operator")
	initOperator, err := setup(wContext)
	if err != nil {
		return fmt.Errorf("error setting up scc operator: %s", err.Error())
	}

	// will be skipped on subsequent starts of the operator
	_, err = initOperator.maybeFirstInit()
	if err != nil {
		return fmt.Errorf("error creating first-start `Registration`: %s", err.Error())
	}

	logrus.Info("[scc-operator] Setup controllers here")
	registration.Register(
		ctx,
		initOperator.registrations,
		initOperator.activations,
		initOperator.configMaps,
		initOperator.secrets,
		initOperator.systemInformation,
	)
	activation.Register(
		ctx,
		initOperator.activations,
		initOperator.secrets,
		initOperator.systemInformation,
	)

	// TODO: Somewhere in operator, or in registration controller, the current Registration needs to be revalidated every 24 hours

	return nil
}
