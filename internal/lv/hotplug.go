package lv

import (
	"fmt"

	"libvirt.org/go/libvirt"
)

// AttachDisk hot-plugs a qcow2 disk to a running domain. The disk is
// attached as a virtio device with the given target (vdb, vdc, …) and
// backed by the file at `sourcePath`.
//
// Flags: AFFECT_LIVE so it takes effect immediately, AFFECT_CONFIG so
// it persists across reboots.
func (c *Client) AttachDisk(domain, sourcePath, target string) error {
	xml := fmt.Sprintf(`<disk type='file' device='disk'>
  <driver name='qemu' type='qcow2' discard='unmap'/>
  <source file='%s'/>
  <target dev='%s' bus='virtio'/>
</disk>`, xmlEscape(sourcePath), xmlEscape(target))
	return c.withDomain(domain, func(d *libvirt.Domain) error {
		return d.AttachDeviceFlags(xml,
			libvirt.DOMAIN_DEVICE_MODIFY_LIVE|libvirt.DOMAIN_DEVICE_MODIFY_CONFIG)
	})
}

// DetachDisk hot-removes a disk from a running domain by target name
// (vdb, vdc, …). Persists the removal in the config.
func (c *Client) DetachDisk(domain, target string) error {
	xml := fmt.Sprintf(`<disk type='file' device='disk'>
  <target dev='%s'/>
</disk>`, xmlEscape(target))
	return c.withDomain(domain, func(d *libvirt.Domain) error {
		return d.DetachDeviceFlags(xml,
			libvirt.DOMAIN_DEVICE_MODIFY_LIVE|libvirt.DOMAIN_DEVICE_MODIFY_CONFIG)
	})
}

// AttachNIC hot-plugs a virtual NIC connected to the given libvirt
// network (e.g. "default"). Model is virtio; MAC is auto-generated.
func (c *Client) AttachNIC(domain, network string) error {
	xml := fmt.Sprintf(`<interface type='network'>
  <source network='%s'/>
  <model type='virtio'/>
</interface>`, xmlEscape(network))
	return c.withDomain(domain, func(d *libvirt.Domain) error {
		return d.AttachDeviceFlags(xml,
			libvirt.DOMAIN_DEVICE_MODIFY_LIVE|libvirt.DOMAIN_DEVICE_MODIFY_CONFIG)
	})
}

// DetachNIC hot-removes a NIC from a running domain by MAC address.
func (c *Client) DetachNIC(domain, mac string) error {
	xml := fmt.Sprintf(`<interface type='network'>
  <mac address='%s'/>
</interface>`, xmlEscape(mac))
	return c.withDomain(domain, func(d *libvirt.Domain) error {
		return d.DetachDeviceFlags(xml,
			libvirt.DOMAIN_DEVICE_MODIFY_LIVE|libvirt.DOMAIN_DEVICE_MODIFY_CONFIG)
	})
}
