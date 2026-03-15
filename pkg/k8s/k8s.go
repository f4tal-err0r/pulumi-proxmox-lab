package k8s

import (
	"fmt"

	"github.com/f4tal-err0r/pulumi-proxmox-lab/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumiverse/pulumi-talos/sdk/go/talos/cluster"
	"github.com/pulumiverse/pulumi-talos/sdk/go/talos/machine"
)

// NodeConfig pairs VM provisioning args with the static IP assigned to that node.
// Talos boots into maintenance mode; the IP must be known ahead of time
// (DHCP reservation or baked into the Talos image via installer config).
type NodeConfig struct {
	providers.VMArgs
	IP string
}

// ClusterArgs fully describes the desired Talos cluster.
type ClusterArgs struct {
	// ClusterName is used as the Talos cluster name and in kubeconfig.
	ClusterName string

	// ControlPlanes lists the control plane nodes. Use 1 or 3+ for HA.
	ControlPlanes []NodeConfig

	// Workers lists the worker nodes (may be empty for single-node setups).
	Workers []NodeConfig

	// VIP is the control plane virtual IP (or the sole CP node IP if not using HA).
	// This becomes the Kubernetes API endpoint: https://VIP:6443.
	VIP string

	// TalosVersion is the Talos release to target (e.g. "v1.7.6").
	TalosVersion string

	// KubernetesVersion is the Kubernetes version to deploy (e.g. "1.30.0").
	KubernetesVersion string
}

// Cluster holds the provisioned Pulumi resources for a Talos cluster.
type Cluster struct {
	ControlPlaneNodes []pulumi.Resource
	WorkerNodes       []pulumi.Resource
	Secrets           *machine.Secrets
	Kubeconfig        *cluster.Kubeconfig
}

// Deploy provisions a Talos cluster on the given provider.
//
// Resource ordering:
//  1. VMs are created via the provider interface.
//  2. Talos secrets (PKI + bootstrap tokens) are generated once for the cluster.
//  3. Machine configs are generated and applied to each node (depends on VM).
//  4. etcd is bootstrapped on the first control plane (depends on all CP applies).
//  5. Kubeconfig is fetched (depends on bootstrap).
func Deploy(ctx *pulumi.Context, p providers.Provider, args ClusterArgs) (*Cluster, error) {
	if err := validateArgs(args); err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("https://%s:6443", args.VIP)
	c := &Cluster{}

	// ── 1. Create VMs ────────────────────────────────────────────────────────

	var allCPDeps []pulumi.Resource
	for i, node := range args.ControlPlanes {
		vmArgs := node.VMArgs
		if len(args.ControlPlanes) > 1 {
			vmArgs.Name = fmt.Sprintf("%s-cp-%d", args.ClusterName, i+1)
		} else {
			vmArgs.Name = fmt.Sprintf("%s-cp", args.ClusterName)
		}
		r, err := p.CreateVM(ctx, vmArgs)
		if err != nil {
			return nil, fmt.Errorf("error creating control plane node %d: %w", i+1, err)
		}
		c.ControlPlaneNodes = append(c.ControlPlaneNodes, r)
		allCPDeps = append(allCPDeps, r)
	}

	for i, node := range args.Workers {
		vmArgs := node.VMArgs
		vmArgs.Name = fmt.Sprintf("%s-worker-%d", args.ClusterName, i+1)
		r, err := p.CreateVM(ctx, vmArgs)
		if err != nil {
			return nil, fmt.Errorf("error creating worker node %d: %w", i+1, err)
		}
		c.WorkerNodes = append(c.WorkerNodes, r)
	}

	// ── 2. Generate cluster secrets (PKI, bootstrap tokens) ──────────────────

	secrets, err := machine.NewSecrets(ctx, fmt.Sprintf("%s-secrets", args.ClusterName), &machine.SecretsArgs{
		TalosVersion: pulumi.StringPtr(args.TalosVersion),
	})
	if err != nil {
		return nil, fmt.Errorf("error generating cluster secrets: %w", err)
	}
	c.Secrets = secrets

	// ── 3. Apply machine configs ──────────────────────────────────────────────

	var cpApplyDeps []pulumi.Resource

	for i, node := range args.ControlPlanes {
		cpConfig := machine.GetConfigurationOutput(ctx, machine.GetConfigurationOutputArgs{
			ClusterName:       pulumi.String(args.ClusterName),
			ClusterEndpoint:   pulumi.String(endpoint),
			MachineType:       pulumi.String("controlplane"),
			MachineSecrets:    secrets.MachineSecrets,
			TalosVersion:      pulumi.StringPtr(args.TalosVersion),
			KubernetesVersion: pulumi.StringPtr(args.KubernetesVersion),
		})

		applyName := fmt.Sprintf("%s-cp-%d-config-apply", args.ClusterName, i+1)
		apply, err := machine.NewConfigurationApply(ctx, applyName, &machine.ConfigurationApplyArgs{
			ClientConfiguration:       secrets.ClientConfiguration,
			MachineConfigurationInput: cpConfig.MachineConfiguration(),
			Node:                      pulumi.String(node.IP),
			Endpoint:                  pulumi.StringPtr(node.IP),
			ApplyMode:                 pulumi.StringPtr("auto"),
		}, pulumi.DependsOn([]pulumi.Resource{c.ControlPlaneNodes[i]}))
		if err != nil {
			return nil, fmt.Errorf("error applying config to control plane node %d: %w", i+1, err)
		}
		cpApplyDeps = append(cpApplyDeps, apply)
	}

	for i, node := range args.Workers {
		workerConfig := machine.GetConfigurationOutput(ctx, machine.GetConfigurationOutputArgs{
			ClusterName:       pulumi.String(args.ClusterName),
			ClusterEndpoint:   pulumi.String(endpoint),
			MachineType:       pulumi.String("worker"),
			MachineSecrets:    secrets.MachineSecrets,
			TalosVersion:      pulumi.StringPtr(args.TalosVersion),
			KubernetesVersion: pulumi.StringPtr(args.KubernetesVersion),
		})

		applyName := fmt.Sprintf("%s-worker-%d-config-apply", args.ClusterName, i+1)
		_, err := machine.NewConfigurationApply(ctx, applyName, &machine.ConfigurationApplyArgs{
			ClientConfiguration:       secrets.ClientConfiguration,
			MachineConfigurationInput: workerConfig.MachineConfiguration(),
			Node:                      pulumi.String(node.IP),
			Endpoint:                  pulumi.StringPtr(node.IP),
			ApplyMode:                 pulumi.StringPtr("auto"),
		}, pulumi.DependsOn([]pulumi.Resource{c.WorkerNodes[i]}))
		if err != nil {
			return nil, fmt.Errorf("error applying config to worker node %d: %w", i+1, err)
		}
	}

	// ── 4. Bootstrap etcd on the first control plane ─────────────────────────

	bootstrap, err := machine.NewBootstrap(ctx, fmt.Sprintf("%s-bootstrap", args.ClusterName), &machine.BootstrapArgs{
		ClientConfiguration: secrets.ClientConfiguration,
		Node:                pulumi.String(args.ControlPlanes[0].IP),
		Endpoint:            pulumi.StringPtr(args.ControlPlanes[0].IP),
	}, pulumi.DependsOn(cpApplyDeps))
	if err != nil {
		return nil, fmt.Errorf("error bootstrapping cluster: %w", err)
	}

	// ── 5. Fetch kubeconfig ───────────────────────────────────────────────────

	// ClientConfiguration must be adapted from machine → cluster package types.
	kubeconfigClientConfig := secrets.ClientConfiguration.ApplyT(
		func(cc machine.ClientConfiguration) cluster.KubeconfigClientConfiguration {
			return cluster.KubeconfigClientConfiguration{
				CaCertificate:     cc.CaCertificate,
				ClientCertificate: cc.ClientCertificate,
				ClientKey:         cc.ClientKey,
			}
		},
	).(cluster.KubeconfigClientConfigurationOutput)

	kubeconfig, err := cluster.NewKubeconfig(ctx, fmt.Sprintf("%s-kubeconfig", args.ClusterName), &cluster.KubeconfigArgs{
		ClientConfiguration: kubeconfigClientConfig,
		Node:                pulumi.String(args.ControlPlanes[0].IP),
		Endpoint:            pulumi.StringPtr(args.VIP),
	}, pulumi.DependsOn([]pulumi.Resource{bootstrap}))
	if err != nil {
		return nil, fmt.Errorf("error fetching kubeconfig: %w", err)
	}
	c.Kubeconfig = kubeconfig

	ctx.Export("clusterEndpoint", pulumi.String(endpoint))
	ctx.Export("kubeconfig", pulumi.ToSecret(kubeconfig.KubeconfigRaw))

	return c, nil
}

func validateArgs(args ClusterArgs) error {
	if args.ClusterName == "" {
		return fmt.Errorf("ClusterName is required")
	}
	if len(args.ControlPlanes) == 0 {
		return fmt.Errorf("at least one control plane node is required")
	}
	if args.VIP == "" {
		return fmt.Errorf("VIP is required")
	}
	if args.TalosVersion == "" {
		return fmt.Errorf("TalosVersion is required (e.g. \"v1.7.6\")")
	}
	if args.KubernetesVersion == "" {
		return fmt.Errorf("KubernetesVersion is required (e.g. \"1.30.0\")")
	}
	for i, cp := range args.ControlPlanes {
		if cp.IP == "" {
			return fmt.Errorf("control plane node %d is missing an IP", i+1)
		}
	}
	for i, w := range args.Workers {
		if w.IP == "" {
			return fmt.Errorf("worker node %d is missing an IP", i+1)
		}
	}
	return nil
}
