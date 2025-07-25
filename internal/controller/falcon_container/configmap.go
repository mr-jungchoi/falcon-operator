package falcon

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	falconv1alpha1 "github.com/crowdstrike/falcon-operator/api/falcon/v1alpha1"
	"github.com/crowdstrike/falcon-operator/internal/controller/assets"
	"github.com/crowdstrike/falcon-operator/pkg/common"
	"github.com/crowdstrike/falcon-operator/pkg/falcon_api"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *FalconContainerReconciler) reconcileRegistryCABundleConfigMap(ctx context.Context, log logr.Logger, falconContainer *falconv1alpha1.FalconContainer) (*corev1.ConfigMap, error) {
	return r.reconcileGenericConfigMap(registryCABundleConfigMapName, r.newCABundleConfigMap, ctx, log, falconContainer)
}

func (r *FalconContainerReconciler) reconcileConfigMap(ctx context.Context, log logr.Logger, falconContainer *falconv1alpha1.FalconContainer) (*corev1.ConfigMap, error) {
	return r.reconcileGenericConfigMap(injectorConfigMapName, r.newConfigMap, ctx, log, falconContainer)
}

func (r *FalconContainerReconciler) reconcileGenericConfigMap(name string, genFunc func(context.Context, logr.Logger, *falconv1alpha1.FalconContainer) (*corev1.ConfigMap, error), ctx context.Context, log logr.Logger, falconContainer *falconv1alpha1.FalconContainer) (*corev1.ConfigMap, error) {
	configMap, err := genFunc(ctx, log, falconContainer)
	if err != nil {
		return configMap, fmt.Errorf("unable to render expected configmap: %v", err)
	}
	existingConfigMap := &corev1.ConfigMap{}
	err = common.GetNamespacedObject(ctx, r.Client, r.Reader, types.NamespacedName{Name: name, Namespace: falconContainer.Spec.InstallNamespace}, existingConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = ctrl.SetControllerReference(falconContainer, configMap, r.Scheme); err != nil {
				return &corev1.ConfigMap{}, fmt.Errorf("unable to set controller reference on config map %s: %v", configMap.ObjectMeta.Name, err)
			}
			return configMap, r.Create(ctx, log, falconContainer, configMap)
		}
		return &corev1.ConfigMap{}, fmt.Errorf("unable to query existing config map %s: %v", name, err)
	}
	if reflect.DeepEqual(configMap.Data, existingConfigMap.Data) {
		return existingConfigMap, nil
	}
	existingConfigMap.Data = configMap.Data
	return existingConfigMap, r.Update(ctx, log, falconContainer, existingConfigMap)

}

func (r *FalconContainerReconciler) newCABundleConfigMap(ctx context.Context, log logr.Logger, falconContainer *falconv1alpha1.FalconContainer) (*corev1.ConfigMap, error) {
	data := make(map[string]string)
	if falconContainer.Spec.Registry.TLS.CACertificate != "" {
		data["tls.crt"] = string(common.DecodeBase64Interface(falconContainer.Spec.Registry.TLS.CACertificate))

		return assets.SensorConfigMap(registryCABundleConfigMapName, falconContainer.Spec.InstallNamespace, common.FalconSidecarSensor, data), nil
	}
	return &corev1.ConfigMap{}, fmt.Errorf("unable to determine contents of Registry TLS CACertificate attribute")
}

func (r *FalconContainerReconciler) newConfigMap(ctx context.Context, log logr.Logger, falconContainer *falconv1alpha1.FalconContainer) (*corev1.ConfigMap, error) {
	data := common.MakeSensorEnvMap(falconContainer.Spec.Falcon)
	data["CP_NAMESPACE"] = falconContainer.Spec.InstallNamespace
	data["FALCON_INJECTOR_LISTEN_PORT"] = strconv.Itoa(int(*falconContainer.Spec.Injector.ListenPort))

	imageUri, err := r.imageUri(ctx, falconContainer)
	if err != nil {
		log.Error(err, "unable to determine falcon-container image URI")
	} else {
		data["FALCON_IMAGE"] = imageUri
	}

	data["FALCON_IMAGE_PULL_POLICY"] = string(falconContainer.Spec.Injector.ImagePullPolicy)

	data["FALCON_IMAGE_PULL_SECRET"] = falconContainer.Spec.Injector.ImagePullSecretName

	if falconContainer.Spec.Injector.DisableDefaultPodInjection {
		data["INJECTION_DEFAULT_DISABLED"] = "T"
	}

	if falconContainer.Spec.Injector.AlternateMountPath {
		data["FALCON_MOUNT_ENABLED"] = "T"
	}

	cid := ""
	if falconContainer.Spec.Falcon.CID != nil {
		cid = *falconContainer.Spec.Falcon.CID
	}

	if cid == "" && falconContainer.Spec.FalconAPI != nil {
		cid, err = falcon_api.FalconCID(ctx, falconContainer.Spec.FalconAPI.CID, falconContainer.Spec.FalconAPI.ApiConfig())
		if err != nil {
			return &corev1.ConfigMap{}, fmt.Errorf("unable to determine Falcon customer ID (CID): %v", err)
		}
	}
	data["FALCONCTL_OPT_CID"] = cid

	if falconContainer.Spec.Injector.LogVolume != nil {
		vol, err := common.EncodeBase64Interface(*falconContainer.Spec.Injector.LogVolume)
		if err != nil {
			log.Error(err, "unable to base64 encode log volume")
		} else {
			data["FALCON_LOG_VOLUME"] = vol
		}
	}

	if falconContainer.Spec.Injector.SensorResources != nil {
		resources, err := common.EncodeBase64Interface(*falconContainer.Spec.Injector.SensorResources)
		if err != nil {
			log.Error(err, "unable to base64 encode falcon resources")
		} else {
			data["FALCON_RESOURCES"] = resources
		}
	}

	if falconContainer.Spec.Injector.AdditionalEnvironmentVariables != nil {
		for k, v := range *falconContainer.Spec.Injector.AdditionalEnvironmentVariables {
			data[strings.ToUpper(k)] = v
		}
	}

	return assets.SensorConfigMap(injectorConfigMapName, falconContainer.Spec.InstallNamespace, common.FalconSidecarSensor, data), nil
}
