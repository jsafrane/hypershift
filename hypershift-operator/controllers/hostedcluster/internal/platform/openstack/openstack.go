package openstack

import (
	"context"
	"errors"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	"github.com/openshift/hypershift/api/util/ipnet"
	"github.com/openshift/hypershift/support/images"
	"github.com/openshift/hypershift/support/upsert"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sutilspointer "k8s.io/utils/pointer"

	capo "sigs.k8s.io/cluster-api-provider-openstack/api/v1beta1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/hypershift/control-plane-operator/controllers/hostedcontrolplane/cloud/openstack"
)

const (
	defaultCIDRBlock = "10.0.0.0/16"
)

type OpenStack struct {
	capiProviderImage string
}

func New(capiProviderImage string) *OpenStack {
	return &OpenStack{
		capiProviderImage: capiProviderImage,
	}
}

func (a OpenStack) ReconcileCAPIInfraCR(ctx context.Context, client client.Client, createOrUpdate upsert.CreateOrUpdateFN, hcluster *hyperv1.HostedCluster,
	controlPlaneNamespace string, apiEndpoint hyperv1.APIEndpoint) (client.Object, error) {
	openStackCluster := &capo.OpenStackCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hcluster.Name,
			Namespace: controlPlaneNamespace,
		},
	}
	openStackPlatform := hcluster.Spec.Platform.OpenStack
	if openStackPlatform == nil {
		return nil, fmt.Errorf("failed to reconcile OpenStack CAPI cluster, empty OpenStack platform spec")
	}

	openStackCluster.Spec.IdentityRef = capo.OpenStackIdentityReference(openStackPlatform.IdentityRef)
	if _, err := createOrUpdate(ctx, client, openStackCluster, func() error {
		reconcileOpenStackClusterSpec(hcluster, &openStackCluster.Spec, apiEndpoint)
		return nil
	}); err != nil {
		return nil, err
	}
	return openStackCluster, nil
}

func reconcileOpenStackClusterSpec(hcluster *hyperv1.HostedCluster, openStackClusterSpec *capo.OpenStackClusterSpec, apiEndpoint hyperv1.APIEndpoint) {
	openStackPlatform := hcluster.Spec.Platform.OpenStack

	openStackClusterSpec.ControlPlaneEndpoint = &capiv1.APIEndpoint{
		Host: apiEndpoint.Host,
		Port: apiEndpoint.Port,
	}

	if len(openStackPlatform.Subnets) > 0 {
		openStackClusterSpec.Subnets = make([]capo.SubnetParam, len(openStackPlatform.Subnets))
		for i := range openStackPlatform.Subnets {
			subnet := openStackPlatform.Subnets[i]
			openStackClusterSpec.Subnets[i] = capo.SubnetParam{ID: subnet.ID}
			subnetFilter := subnet.Filter
			if subnetFilter != nil {
				openStackClusterSpec.Subnets[i].Filter = &capo.SubnetFilter{
					Name:                subnetFilter.Name,
					Description:         subnetFilter.Description,
					ProjectID:           subnetFilter.ProjectID,
					IPVersion:           subnetFilter.IPVersion,
					GatewayIP:           subnetFilter.GatewayIP,
					CIDR:                subnetFilter.CIDR,
					IPv6AddressMode:     subnetFilter.IPv6AddressMode,
					IPv6RAMode:          subnetFilter.IPv6RAMode,
					FilterByNeutronTags: createCAPOFilterTags(subnetFilter.Tags, subnetFilter.TagsAny, subnetFilter.NotTags, subnetFilter.NotTagsAny),
				}
			}
		}
	} else {
		var machineNetworks []hyperv1.MachineNetworkEntry
		// If no MachineNetwork is provided, use a default CIDR block.
		// Note: The default is required for now because there is no CLI option to set the MachineNetwork.
		// See https://github.com/openshift/hypershift/pull/4287
		if hcluster.Spec.Networking.MachineNetwork == nil || len(hcluster.Spec.Networking.MachineNetwork) == 0 {
			machineNetworks = []hyperv1.MachineNetworkEntry{{CIDR: *ipnet.MustParseCIDR(defaultCIDRBlock)}}
		} else {
			machineNetworks = hcluster.Spec.Networking.MachineNetwork
		}
		openStackClusterSpec.ManagedSubnets = make([]capo.SubnetSpec, len(machineNetworks))
		// Only one Subnet is supported in CAPO
		openStackClusterSpec.ManagedSubnets[0] = capo.SubnetSpec{
			CIDR: machineNetworks[0].CIDR.String(),
		}
		for i := range openStackPlatform.ManagedSubnets {
			openStackClusterSpec.ManagedSubnets[i].DNSNameservers = openStackPlatform.ManagedSubnets[i].DNSNameservers
			allocationPools := openStackPlatform.ManagedSubnets[i].AllocationPools
			openStackClusterSpec.ManagedSubnets[i].AllocationPools = make([]capo.AllocationPool, len(allocationPools))
			for j := range allocationPools {
				openStackClusterSpec.ManagedSubnets[i].AllocationPools[j] = capo.AllocationPool{
					Start: allocationPools[j].Start,
					End:   allocationPools[j].End,
				}
			}
		}
	}
	if openStackPlatform.Router != nil {
		openStackClusterSpec.Router = &capo.RouterParam{ID: openStackPlatform.Router.ID}
		if openStackPlatform.Router.Filter != nil {
			routerFilter := openStackPlatform.Router.Filter
			openStackClusterSpec.Router.Filter = &capo.RouterFilter{
				Name:                routerFilter.Name,
				Description:         routerFilter.Description,
				ProjectID:           routerFilter.ProjectID,
				FilterByNeutronTags: createCAPOFilterTags(routerFilter.Tags, routerFilter.TagsAny, routerFilter.NotTags, routerFilter.NotTagsAny),
			}

		}
	}
	if openStackPlatform.Network != nil {
		openStackClusterSpec.Network = &capo.NetworkParam{ID: openStackPlatform.Network.ID}
		if openStackPlatform.Network.Filter != nil {
			openStackClusterSpec.Network.Filter = createCAPONetworkFilter(openStackPlatform.Network.Filter)
		}
	}
	if openStackPlatform.NetworkMTU != nil {
		openStackClusterSpec.NetworkMTU = openStackPlatform.NetworkMTU
	}
	if openStackPlatform.ExternalNetwork != nil {
		openStackClusterSpec.ExternalNetwork = &capo.NetworkParam{ID: openStackPlatform.ExternalNetwork.ID}
		if openStackPlatform.ExternalNetwork.Filter != nil {
			openStackClusterSpec.ExternalNetwork.Filter = createCAPONetworkFilter(openStackPlatform.ExternalNetwork.Filter)
		}
	}
	if openStackPlatform.DisableExternalNetwork != nil {
		openStackClusterSpec.DisableExternalNetwork = openStackPlatform.DisableExternalNetwork
	}
	openStackClusterSpec.ManagedSecurityGroups = &capo.ManagedSecurityGroups{}
	openStackClusterSpec.DisableAPIServerFloatingIP = k8sutilspointer.BoolPtr(true)
	openStackClusterSpec.Tags = openStackPlatform.Tags
}

func convertHypershiftTagToCAPOTag(tags []hyperv1.NeutronTag) []capo.NeutronTag {
	var capoTags []capo.NeutronTag
	for i := range tags {
		capoTags = append(capoTags, capo.NeutronTag(tags[i]))
	}
	return capoTags
}

func createCAPOFilterTags(tags, tagsAny, NotTags, NotTagsAny []hyperv1.NeutronTag) capo.FilterByNeutronTags {
	return capo.FilterByNeutronTags{
		Tags:       convertHypershiftTagToCAPOTag(tags),
		TagsAny:    convertHypershiftTagToCAPOTag(tagsAny),
		NotTags:    convertHypershiftTagToCAPOTag(NotTags),
		NotTagsAny: convertHypershiftTagToCAPOTag(NotTagsAny),
	}
}

func createCAPONetworkFilter(filter *hyperv1.NetworkFilter) *capo.NetworkFilter {
	return &capo.NetworkFilter{
		Name:                filter.Name,
		Description:         filter.Description,
		ProjectID:           filter.ProjectID,
		FilterByNeutronTags: createCAPOFilterTags(filter.Tags, filter.TagsAny, filter.NotTags, filter.NotTagsAny),
	}
}

func (a OpenStack) CAPIProviderDeploymentSpec(hcluster *hyperv1.HostedCluster, _ *hyperv1.HostedControlPlane) (*appsv1.DeploymentSpec, error) {
	image := a.capiProviderImage
	if envImage := os.Getenv(images.OpenStackCAPIProviderEnvVar); len(envImage) > 0 {
		image = envImage
	}
	if override, ok := hcluster.Annotations[hyperv1.ClusterAPIOpenStackProviderImage]; ok {
		image = override
	}
	defaultMode := int32(0640)
	return &appsv1.DeploymentSpec{
		Replicas: k8sutilspointer.Int32(1),
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: "capi-webhooks-tls",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								DefaultMode: &defaultMode,
								SecretName:  "capi-webhooks-tls",
							},
						},
					},
				},
				Containers: []corev1.Container{{
					Name:            "manager",
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/manager"},
					Args: []string{
						"--namespace=$(MY_NAMESPACE)",
						"--leader-elect",
						"--metrics-bind-addr=127.0.0.1:8080",
						"--v=2",
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "healthz",
							ContainerPort: 9440,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.FromString("healthz"),
							},
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/readyz",
								Port: intstr.FromString("healthz"),
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "capi-webhooks-tls",
							ReadOnly:  true,
							MountPath: "/tmp/k8s-webhook-server/serving-certs",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name: "MY_NAMESPACE",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.namespace",
								},
							},
						},
					},
				}},
			}}}, nil
}

func (a OpenStack) ReconcileCredentials(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN, hcluster *hyperv1.HostedCluster, controlPlaneNamespace string) error {
	return errors.Join(
		a.reconcileCloudsYaml(ctx, c, createOrUpdate, controlPlaneNamespace, hcluster.Namespace, hcluster.Spec.Platform.OpenStack.IdentityRef.Name),
		a.reconcileCACert(ctx, c, createOrUpdate, controlPlaneNamespace, hcluster.Namespace, hcluster.Spec.Platform.OpenStack.IdentityRef.Name),
	)
}

func (a OpenStack) reconcileCloudsYaml(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN, controlPlaneNamespace string, clusterNamespace string, identityRefName string) error {
	var source corev1.Secret

	// Sync user cloud.conf secret
	name := client.ObjectKey{Namespace: clusterNamespace, Name: identityRefName}
	if err := c.Get(ctx, name, &source); err != nil {
		return fmt.Errorf("failed to get secret %s: %w", name, err)
	}

	clouds := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNamespace, Name: identityRefName}}
	_, err := createOrUpdate(ctx, c, clouds, func() error {
		if clouds.Data == nil {
			clouds.Data = map[string][]byte{}
		}
		clouds.Data["clouds.yaml"] = source.Data["clouds.yaml"] // TODO(emilien): Proper missing key handling.
		clouds.Data["clouds.conf"] = source.Data["clouds.conf"] // TODO(emilien): Could we just generate this from clouds.yaml here?
		if _, ok := source.Data["cacert"]; ok {
			clouds.Data["cacert"] = source.Data["cacert"]
		}
		return nil
	})

	return err
}

func (a OpenStack) reconcileCACert(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN, controlPlaneNamespace string, clusterNamespace string, secretName string) error {
	credentialsSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: clusterNamespace, Name: secretName}}
	if err := c.Get(ctx, client.ObjectKey{Namespace: clusterNamespace, Name: secretName}, credentialsSecret); err != nil {
		return fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	caCertData := openstack.GetCACertFromCredentialsSecret(credentialsSecret)
	if caCertData == nil {
		return nil
	}

	caCert := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNamespace, Name: "openstack-ca"}}
	if _, err := createOrUpdate(ctx, c, caCert, func() error {
		if caCert.Data == nil {
			caCert.Data = map[string][]byte{}
		}
		caCert.Data["ca.pem"] = caCertData // TODO(emilien): Proper missing key handling, naming.
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (a OpenStack) ReconcileSecretEncryption(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN, hcluster *hyperv1.HostedCluster, controlPlaneNamespace string) error {
	return nil
}

func (a OpenStack) CAPIProviderPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{"ipam.cluster.x-k8s.io"},
			Resources: []string{"ipaddressclaims", "ipaddressclaims/status"},
			Verbs:     []string{rbacv1.VerbAll},
		},
		{
			APIGroups: []string{"ipam.cluster.x-k8s.io"},
			Resources: []string{"ipaddresses", "ipaddresses/status"},
			Verbs:     []string{"create", "delete", "get", "list", "update", "watch"},
		},
	}
}

func (a OpenStack) DeleteCredentials(ctx context.Context, c client.Client, hcluster *hyperv1.HostedCluster, controlPlaneNamespace string) error {
	return nil
}
