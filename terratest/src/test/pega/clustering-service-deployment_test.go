package pega

import (
	"fmt"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/util/intstr"
	"path/filepath"
	"strings"
	"testing"
	"strconv"
)

func TestClusteringServiceDeployment(t *testing.T) {
    SetUp(t, false)
}

func TestClusteringServiceDeploymentSSL(t *testing.T) {
    SetUp(t, true)
}

func SetUp(t *testing.T, sslEnabled bool) {
	var supportedVendors = []string{"k8s", "openshift", "eks", "gke", "aks", "pks"}
	var supportedOperations = []string{"deploy", "install-deploy"}

	helmChartPath, err := filepath.Abs(PegaHelmChartPath)
	require.NoError(t, err)

	for _, vendor := range supportedVendors {

		for _, operation := range supportedOperations {

			fmt.Println(vendor + "-" + operation)

			var options = &helm.Options{
				SetValues: map[string]string{
					"global.provider":                    vendor,
					"global.actions.execute":             operation,
					"hazelcast.clusteringServiceEnabled": "true",
					"hazelcast.encryption.enabled": strconv.FormatBool(sslEnabled),
				},
			}

			yamlContent := RenderTemplate(t, options, helmChartPath, []string{"charts/hazelcast/templates/clustering-service-deployment.yaml"})
			VerifyClusteringServiceDeployment(t, yamlContent, sslEnabled)

		}
	}
}

func VerifyClusteringServiceDeployment(t *testing.T, yamlContent string, sslEnabled bool) {
	var statefulsetObj appsv1beta2.StatefulSet
	statefulSlice := strings.Split(yamlContent, "---")
	for index, statefulInfo := range statefulSlice {
		if index >= 1 {
			UnmarshalK8SYaml(t, statefulInfo, &statefulsetObj)
			require.Equal(t, *statefulsetObj.Spec.Replicas, int32(3))
			require.Equal(t, statefulsetObj.Spec.ServiceName, "clusteringservice-service")
			statefulsetSpec := statefulsetObj.Spec.Template.Spec
			require.Equal(t, "/hazelcast/health/ready", statefulsetSpec.Containers[0].LivenessProbe.HTTPGet.Path)
			require.Equal(t, "/hazelcast/health/ready", statefulsetSpec.Containers[0].ReadinessProbe.HTTPGet.Path)
			require.Equal(t, "1", statefulsetSpec.Containers[0].Resources.Requests.Cpu().String())
			require.Equal(t, "1Gi", statefulsetSpec.Containers[0].Resources.Requests.Memory().String())
			require.Equal(t, statefulsetSpec.Volumes[0].Name, "logs")
			require.Equal(t, statefulsetSpec.Volumes[1].Name, "hazelcast-volume-credentials")
			require.Equal(t, statefulsetSpec.Volumes[1].Projected.Sources[0].Secret.Name, "pega-hz-secret")
			require.Equal(t, statefulsetSpec.Containers[0].VolumeMounts[0].Name, "logs")
			require.Equal(t, statefulsetSpec.Containers[0].VolumeMounts[0].MountPath, "/opt/hazelcast/logs")
			require.Equal(t, statefulsetSpec.Containers[0].VolumeMounts[1].Name, "hazelcast-volume-credentials")
			require.Equal(t, statefulsetSpec.Containers[0].VolumeMounts[1].MountPath, "/opt/hazelcast/secrets")

			if sslEnabled {
                require.Equal(t, intstr.FromInt(8080), statefulsetSpec.Containers[0].LivenessProbe.HTTPGet.Port)
                require.Equal(t, intstr.FromInt(8080), statefulsetSpec.Containers[0].ReadinessProbe.HTTPGet.Port)
                require.Equal(t, statefulsetSpec.Containers[0].VolumeMounts[2].Name, "hz-encryption-secrets")
                require.Equal(t, statefulsetSpec.Containers[0].VolumeMounts[2].MountPath, "/opt/hazelcast/certs")

			} else {
                require.Equal(t, intstr.FromInt(5701), statefulsetSpec.Containers[0].LivenessProbe.HTTPGet.Port)
                require.Equal(t, intstr.FromInt(5701), statefulsetSpec.Containers[0].ReadinessProbe.HTTPGet.Port)
			}
		}
	}
}

