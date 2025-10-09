package main

import "github.com/f4tal-err0r/providers/proxmox"

var VMs = []proxmox.VMArgs{
	{
		Name:       "bastion",
		NodeName:   "pve",
		Cores:      2,
		Memory:     4096,
		DiskSizeGB: 40,
		Bridge:     "vmbr0",
	},
}
