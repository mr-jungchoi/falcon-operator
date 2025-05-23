package falcon

import (
	"context"
	"fmt"
	"reflect"

	falconv1alpha1 "github.com/crowdstrike/falcon-operator/api/falcon/v1alpha1"
	"github.com/crowdstrike/falcon-operator/internal/controller/assets"
	"github.com/crowdstrike/falcon-operator/pkg/common"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *FalconContainerReconciler) reconcileService(ctx context.Context, log logr.Logger, falconContainer *falconv1alpha1.FalconContainer) (*corev1.Service, error) {
	selector := map[string]string{common.FalconComponentKey: common.FalconSidecarSensor}
	service := assets.Service(injectorName, falconContainer.Spec.InstallNamespace, common.FalconSidecarSensor, selector, common.FalconServiceHTTPSName, *falconContainer.Spec.Injector.ListenPort)
	updated := false
	existingService := &corev1.Service{}

	err := common.GetNamespacedObject(ctx, r.Client, r.Reader, types.NamespacedName{Name: injectorName, Namespace: falconContainer.Spec.InstallNamespace}, existingService)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = ctrl.SetControllerReference(falconContainer, service, r.Scheme); err != nil {
				return &corev1.Service{}, fmt.Errorf("unable to set controller reference on service %s: %v", service.ObjectMeta.Name, err)
			}

			return service, r.Create(ctx, log, falconContainer, service)
		}

		return &corev1.Service{}, fmt.Errorf("unable to query existing service %s: %v", injectorName, err)
	}

	if !reflect.DeepEqual(service.Spec.Selector, existingService.Spec.Selector) {
		existingService.Spec.Selector = service.Spec.Selector
		updated = true
	}

	if !reflect.DeepEqual(service.Spec.Ports, existingService.Spec.Ports) {
		existingService.Spec.Ports = service.Spec.Ports
		updated = true
	}

	if updated {
		return existingService, r.Update(ctx, log, falconContainer, existingService)
	}

	return existingService, nil

}
