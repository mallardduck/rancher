package controllers

import (
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/scc/util/jitterbug"
	"k8s.io/apimachinery/pkg/labels"
)

func (h *handler) runRegistration() {
	cfg := setupJitter()
	jitterCheckin := jitterbug.NewJitterChecker(
		&cfg,
		func(nextTrigger, strictDeadline time.Duration) (bool, error) {
			registrationsCacheList, err := h.registrationCache.List(labels.Everything())
			if err != nil {
				h.log.Errorf("Failed to list registrations: %v", err)
				return false, err
			}

			checkInWasTriggered := false
			for _, registrationObj := range registrationsCacheList {
				// TODO(dan) / FIXME this logic is encapsulated in the reconciler.
				// having this loop control whether or not it actually gets sent, doesn't solve the jitter problem since
				// any object update that makes it into the 20-24hour window will immediately requeue it and send it

				// If we're already checking the CRD status in the reconciler to decide what to do, then let's add a status
				// condition to signify that it must send its payload

				registrationHandler := h.prepareHandler(h.systemNamespace, registrationObj.Spec.Mode)

				// Always skip offline mode registrations, or Registrations that haven't progressed to activation
				if registrationObj.Spec.Mode == v1.RegistrationModeOffline ||
					registrationHandler.NeedsRegistration(registrationObj) ||
					registrationObj.Status.ActivationStatus.LastValidatedTS.IsZero() {
					continue
				}

				lastValidated := registrationObj.Status.ActivationStatus.LastValidatedTS

				timeSinceLastValidation := time.Since(lastValidated.Time)
				// If the time since last validation is after the daily trigger (which includes jitter), we revalidate.
				// Also, ensure that when a registration is over the strictDeadline it is checked.
				if timeSinceLastValidation >= nextTrigger || timeSinceLastValidation >= strictDeadline {
					checkInWasTriggered = true
					// TODO (o&b): 95% sure that enqueue alone won't be good enough based on other controller logic.
					// Either we need to adjust that controller logic so enqueue alone is enough, or use `CheckNow`.
					// Seems check now is most simple as it reuses controller logic

					// TODO(dan) What we need to do here is likely update the object status and then when processing errors occur,
					// we need to requeue the object for processing.
					// Likely something mirroring requeue.After() in controller runtime
					h.registrations.Enqueue(registrationObj.Name)
				}
			}

			return checkInWasTriggered, nil
		},
	)
	jitterCheckin.Start()
	jitterCheckin.Run()

}

func setupJitter() jitterbug.Config {
	// Configure jitter based daily revalidation trigger
	jitterbugConfig := jitterbug.Config{
		BaseInterval:    prodBaseCheckin,
		JitterMax:       3,
		JitterMaxScale:  time.Hour,
		PollingInterval: 9 * time.Minute,
	}
	if util.VersionIsDevBuild() {
		jitterbugConfig = jitterbug.Config{
			BaseInterval:    devBaseCheckin,
			JitterMax:       10,
			JitterMaxScale:  time.Minute,
			PollingInterval: 9 * time.Second,
		}
	}
	return jitterbugConfig
}
