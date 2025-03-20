package scc

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/version"
	"github.com/rancher/wrangler/v3/pkg/start"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"

	"github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io"
	"github.com/rancher/rancher/pkg/scc/controllers/registration"
	"github.com/rancher/rancher/pkg/scc/controllers/registrationrequest"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

type sccOperator struct {
	sccFactory        *scc.Factory
	core              *v1core.Factory
	systemInformation *util.RancherSystemInfo
}

func setup(wContext *wrangler.Context) (sccOperator, error) {
	restConfig := wContext.RESTConfig
	registrationSccFactory, err := scc.NewFactoryFromConfig(restConfig)
	if err != nil {
		return sccOperator{}, fmt.Errorf("error building scc controllers: %s", err.Error())
	}

	coreF, err := v1core.NewFactoryFromConfig(restConfig)
	if err != nil {
		return sccOperator{}, fmt.Errorf("error building core controllers: %s", err.Error())
	}

	namespaces := coreF.Core().V1().Namespace()
	kubeSystemNS, err := namespaces.Get("kube-system", metav1.GetOptions{})
	if err != nil {
		// fatal log here, because we need the kube-system ns UID while creating any backup file
		logrus.Fatalf("Error getting namespace kube-system %v", err)
	}

	cattleSystemNS, err := namespaces.Get("cattle-system", metav1.GetOptions{})
	if err != nil {
		// fatal log here, because we need the kube-system ns UID while creating any backup file
		logrus.Fatalf("Error getting namespace cattle-system %v", err)
	}

	// This needs a watcher - if URL is updated this operator needs to be informed
	serverUrl := settings.ServerURL.Get()
	if serverUrl == "" {
		logrus.Fatalf("Missing server url")
	}
	// TODO: also get Node, Sockets, Vcpus, Clusters and watch those

	return sccOperator{
		sccFactory: registrationSccFactory,
		core:       coreF,
		systemInformation: &util.RancherSystemInfo{
			ClusterUuid: uuid.MustParse(string(kubeSystemNS.UID)),
			RancherUuid: uuid.MustParse(string(cattleSystemNS.UID)),
			Url:         serverUrl,
			Version:     version.Version,
		},
	}, nil
}

// maybeFirstInit will check if the initial `RegistrationRequest` seeding values exist
// and if they need to be processed into a new `RegistrationRequest` (used during first boot ever)
func (so *sccOperator) maybeFirstInit() error {
	logrus.Info("SCC controller MaybeFirstInit")
	if strings.EqualFold(settings.SCCFirstStart.Get(), "false") {
		logrus.Warn("Skipping the SCC controller first start; first start already completed previously.")
		return nil
	}

	// Check if the `cattle-system:initial-scc-registration` ConfigMap exists
	// If it does not, then we simply proceed and mark the setting as false
	configMap, err := so.core.Core().V1().ConfigMap().Get("cattle-system", "initial-scc-registration", metav1.GetOptions{})
	if err != nil {
		logrus.Warn("Cannot find initial-scc-registration configmap; it will be skipped")
	} else {
		secretName, mode, err := util.ValidateInitializingConfigMap(configMap)
		if err != nil {
			return err
		}

		newRegistrationRequest := &v1.RegistrationRequest{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "rancher-",
			},
		}
		newRegistrationRequest.Spec.Mode = *mode
		if *mode == v1.Offline {
			newRegistrationRequest.Spec.RegistrationCertificateSecretRef = &corev1.SecretReference{
				Name: secretName,
			}
		} else {
			newRegistrationRequest.Spec.RegistrationCodeSecretRef = &corev1.SecretReference{
				Name: secretName,
			}
		}

		_, err = so.sccFactory.Scc().V1().RegistrationRequest().Create(newRegistrationRequest)
		if err != nil {
			logrus.Errorf("Cannot create registration request; %s", err)
			return err
		}
	}

	// At very end, we will set it to false so this doesn't run again
	if !strings.EqualFold(settings.SCCFirstStart.Get(), "false") {
		if err := settings.SCCFirstStart.Set("false"); err != nil {
			return err
		}
	}

	return nil
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
	err = initOperator.maybeFirstInit()
	if err != nil {
		return fmt.Errorf("error creating first-start `RegistrationRequest`: %s", err.Error())
	}

	logrus.Info("[scc-operator] Setup controllers here")
	registrationrequest.Register(
		ctx,
		initOperator.sccFactory.Scc().V1().RegistrationRequest(),
		initOperator.sccFactory.Scc().V1().Registration(),
		initOperator.core.Core().V1().ConfigMap(),
		initOperator.core.Core().V1().Secret(),
		initOperator.systemInformation,
	)
	registration.Register(
		ctx,
		initOperator.sccFactory.Scc().V1().Registration(),
		initOperator.systemInformation,
	)

	// TODO: verify this is correct
	if err := start.All(ctx, 2, initOperator.sccFactory); err != nil {
		logrus.Fatalf("Error starting: %s", err.Error())
	}

	// TODO: Somewhere in operator, or in registration controller, the current Registration needs to be revalidated every 24 hours

	return nil
}
