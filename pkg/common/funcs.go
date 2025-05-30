package common

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

func InitContainerArgs() []string {
	return []string{
		"-c",
		fmt.Sprintf("echo \"Running %[1]s\"; %[1]s; echo \"Running %[2]s\"; test -f \"%[2]s\" && %[2]s || echo \"%[2]s not found. Skipping.\"", FalconDaemonsetInitBinary, FalconDaemonsetConfigureClusterIdBinary),
	}
}

func InitCleanupArgs() []string {
	return []string{
		"-c",
		fmt.Sprintf("echo \"Running %[1]s\"; %[1]s", FalconDaemonsetCleanupBinary),
	}
}

func CleanupSleep() []string {
	return []string{
		"-c",
		"sleep infinity",
	}
}

func FCAdmissionReviewVersions() []string {
	kubeVersion := GetKubernetesVersion()
	fcArv := []string{"v1"}

	if strings.Compare(kubeVersion.Minor, "22") < 0 {
		fcArv = []string{"v1", "v1beta"}
	}

	return fcArv
}

func GetKubernetesVersion() *version.Info {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	version, err := clientset.DiscoveryClient.ServerVersion()
	if err != nil {
		panic(err.Error())
	}

	return version
}

func EncodedBase64String(data string) []byte {
	base64EncodedData := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(base64EncodedData, []byte(data))
	return base64EncodedData
}

func EncodeBase64Interface(i interface{}) (string, error) {
	buf := bytes.Buffer{}
	b64enc := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := json.NewEncoder(b64enc).Encode(i); err != nil {
		return "", fmt.Errorf("failed to convert to base64 encoding: %v", err)
	}
	if err := b64enc.Close(); err != nil {
		return "", fmt.Errorf("failed to close base64 encoder: %v", err)
	}
	return buf.String(), nil
}

func DecodeBase64Interface(i interface{}) string {
	var str string
	switch v := i.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)

	}
	b64byte, err := base64.StdEncoding.DecodeString(str)
	if err == nil {
		return string(b64byte)
	}
	return str
}

func CleanDecodedBase64(s []byte) []byte {
	re := regexp.MustCompile(`[\t|\n]*`)
	b64byte, err := base64.StdEncoding.DecodeString(string(s))
	if err != nil {
		return []byte(re.ReplaceAllString(string(s), ""))
	}
	return []byte(re.ReplaceAllString(string(b64byte), ""))
}

func MapCopy(src map[string]string, dst map[string]string) map[string]string {
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func CRLabels(instanceName string, instanceKey string, component string) map[string]string {
	return map[string]string{
		FalconInstanceNameKey: instanceName,
		FalconInstanceKey:     instanceKey,
		FalconComponentKey:    component,
		FalconManagedByKey:    FalconManagedByValue,
		FalconProviderKey:     FalconProviderValue,
		FalconPartOfKey:       FalconPartOfValue,
		FalconCreatedKey:      FalconCreatedValue,
	}
}

func AppendUniqueEnvVars(envVars ...[]corev1.EnvVar) []corev1.EnvVar {
	base := []corev1.EnvVar{}
	for _, envVars := range envVars {
		if envVars == nil {
			continue
		}
		for _, envVar := range envVars {
			if !containsEnvVar(base, envVar) {
				base = append(base, envVar)
			}
		}
	}
	return base
}

func containsEnvVar(envVars []corev1.EnvVar, envVar corev1.EnvVar) bool {
	for _, e := range envVars {
		if e.Name == envVar.Name {
			return true
		}
	}
	return false
}

func UpdateEnvVars(envVars []corev1.EnvVar, updateEnvVars []corev1.EnvVar) []corev1.EnvVar {
	for i, envVar := range envVars {
		for _, e := range updateEnvVars {
			if envVar.Name == e.Name {
				if envVar.Value != e.Value {
					envVars[i].Value = e.Value
				}
			}
		}
	}

	return envVars
}

func ImageVersion(image string) *string {
	switch {
	case strings.Contains(image, ":"):
		versionTag := strings.Split(image, ":")
		return &versionTag[1]
	case strings.Contains(image, "@"):
		versionDigest := strings.Split(image, "@")
		return &versionDigest[1]
	default:
		return &image
	}
}

func GetNamespacedObject(ctx context.Context, client client.Client, apiReader client.Reader, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	err := client.Get(ctx, key, obj, opts...)
	if !errors.IsNotFound(err) {
		return err
	}
	return apiReader.Get(ctx, key, obj, opts...)
}
