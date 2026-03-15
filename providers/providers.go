package providers

import "github.com/pulumi/pulumi/sdk/v3/go/pulumi"

// VMArgs are provider-agnostic parameters for creating a VM.
type VMArgs struct {
	Name         string
	NodeName     string
	Cores        int
	Memory       int // in MiB
	DiskSizeGB   int
	Bridge       string
	TemplateVMID int    // template to clone from; 0 means no clone
	TemplateNode string // node where the template lives (defaults to NodeName if empty)
}

// VMInfo represents a discovered VM or template.
type VMInfo struct {
	Name     string
	NodeName string
	VmId     int
	Tags     []string
}

// Provider is the interface all infrastructure providers must implement.
// Methods return pulumi.Resource so implementations are not tied to any
// specific Pulumi provider SDK (e.g. proxmoxve, AWS, GCP).
type Provider interface {
	// CreateVM provisions a new virtual machine and returns the Pulumi resource.
	CreateVM(ctx *pulumi.Context, args VMArgs) (pulumi.Resource, error)

	// ListVMs returns all VMs visible to the provider on the given node.
	// Pass an empty nodeName to list across all nodes.
	ListVMs(ctx *pulumi.Context, nodeName string) ([]VMInfo, error)

	// ListTemplates returns VMs available as templates on the given node.
	// Pass an empty nodeName to search across all nodes.
	ListTemplates(ctx *pulumi.Context, nodeName string) ([]VMInfo, error)
}
