package proxmox

import (
	"fmt"

	"github.com/f4tal-err0r/pulumi-proxmox-lab/providers"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve/vm"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Compile-time check that *Provider satisfies providers.Provider.
var _ providers.Provider = (*Provider)(nil)

type Provider struct {
	*proxmoxve.Provider
}

type ProxmoxCredentials struct {
	Endpoint string
	ApiToken string
}

func NewProxmoxProvider(ctx *pulumi.Context, creds ProxmoxCredentials) (*Provider, error) {
	p, err := proxmoxve.NewProvider(ctx, "proxmoxve", &proxmoxve.ProviderArgs{
		Endpoint: pulumi.StringPtr(creds.Endpoint),
		ApiToken: pulumi.StringPtr(creds.ApiToken),
	})
	if err != nil {
		return nil, fmt.Errorf("error creating proxmox provider: %w", err)
	}
	return &Provider{p}, nil
}

func (p *Provider) CreateVM(ctx *pulumi.Context, args providers.VMArgs) (pulumi.Resource, error) {
	vmArgs := &vm.VirtualMachineArgs{
		NodeName: pulumi.String(args.NodeName),
		Name:     pulumi.String(args.Name),
		Cpu: &vm.VirtualMachineCpuArgs{
			Cores: pulumi.Int(args.Cores),
		},
		Memory: &vm.VirtualMachineMemoryArgs{
			Dedicated: pulumi.Int(args.Memory),
		},
		NetworkDevices: vm.VirtualMachineNetworkDeviceArray{
			&vm.VirtualMachineNetworkDeviceArgs{
				Bridge: pulumi.String(args.Bridge),
				Model:  pulumi.String("virtio"),
			},
		},
	}

	if args.TemplateVMID != 0 {
		cloneNode := args.TemplateNode
		if cloneNode == "" {
			cloneNode = args.NodeName
		}
		vmArgs.Clone = &vm.VirtualMachineCloneArgs{
			VmId:     pulumi.Int(args.TemplateVMID),
			NodeName: pulumi.StringPtr(cloneNode),
			Full:     pulumi.BoolPtr(true),
		}
	} else {
		vmArgs.Disks = vm.VirtualMachineDiskArray{
			&vm.VirtualMachineDiskArgs{
				Interface:   pulumi.String("virtio0"),
				DatastoreId: pulumi.String("local-lvm"),
				Size:        pulumi.Int(args.DiskSizeGB),
			},
		}
	}

	v, err := vm.NewVirtualMachine(ctx, args.Name, vmArgs, pulumi.Provider(p.Provider))
	if err != nil {
		return nil, fmt.Errorf("error creating VM %q: %w", args.Name, err)
	}
	return v, nil
}

// ListVMs returns all VMs on the given node. Pass an empty nodeName to list across all nodes.
func (p *Provider) ListVMs(ctx *pulumi.Context, nodeName string) ([]providers.VMInfo, error) {
	var nodeFilter *string
	if nodeName != "" {
		nodeFilter = &nodeName
	}

	result, err := vm.GetVirtualMachines(ctx, &vm.GetVirtualMachinesArgs{
		NodeName: nodeFilter,
	}, pulumi.Provider(p.Provider))
	if err != nil {
		return nil, fmt.Errorf("error listing VMs: %w", err)
	}

	vms := make([]providers.VMInfo, len(result.Vms))
	for i, v := range result.Vms {
		vms[i] = providers.VMInfo{
			Name:     v.Name,
			NodeName: v.NodeName,
			VmId:     v.VmId,
			Tags:     v.Tags,
		}
	}
	return vms, nil
}

// ListTemplates returns VMs tagged with "template" on the given node.
// Pass an empty nodeName to search across all nodes.
// Note: the SDK does not expose a Template boolean in list results; tagging
// VMs with "template" in Proxmox is the recommended convention to use this filter.
func (p *Provider) ListTemplates(ctx *pulumi.Context, nodeName string) ([]providers.VMInfo, error) {
	var nodeFilter *string
	if nodeName != "" {
		nodeFilter = &nodeName
	}

	result, err := vm.GetVirtualMachines(ctx, &vm.GetVirtualMachinesArgs{
		NodeName: nodeFilter,
		Tags:     []string{"template"},
	}, pulumi.Provider(p.Provider))
	if err != nil {
		return nil, fmt.Errorf("error listing templates: %w", err)
	}

	templates := make([]providers.VMInfo, len(result.Vms))
	for i, v := range result.Vms {
		templates[i] = providers.VMInfo{
			Name:     v.Name,
			NodeName: v.NodeName,
			VmId:     v.VmId,
			Tags:     v.Tags,
		}
	}
	return templates, nil
}
