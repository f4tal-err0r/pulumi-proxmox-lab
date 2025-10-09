package providers

import (
	"github.com/muhlba91/pulumi-proxmoxve/sdk/go/proxmoxve"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ProxmoxCredentials struct {
	Endpoint string
	Username string
	Password string
}

type VMArgs struct {
    Name          string
    NodeName      string
    Cores         int
    Memory        int    // in MiB
    DiskSizeGB    int
    Bridge        string
	TemplateID    string
}



func NewProxmoxProvider(ctx *pulumi.Context, creds ProxmoxCredentials) (*proxmoxve.Provider, error) {
	return proxmoxve.NewProvider(ctx, "proxmoxve", &proxmoxve.ProviderArgs{
		VirtualEnvironment: &proxmoxve.ProviderVirtualEnvironmentArgs{
			Endpoint: pulumi.String(creds.Endpoint),
			Username: pulumi.String(creds.Username),
			Password: pulumi.String(creds.Password),
		},
	})
}

func NewProxmoxVM