package providers

import (
	"github.com/f4tal-err0r/pulumi-lab-live/providers/proxmox"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/go/proxmoxve"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Client struct {
	Type string

	*proxmox.Provider
}

type Provider interface {
	CreateVM(ctx *pulumi.Context, provider *proxmoxve.Provider, args VMArgs)
}
