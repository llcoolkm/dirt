package lv

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"libvirt.org/go/libvirt"
)

// DomainInfo is a structured view of a libvirt domain's XML definition,
// suitable for rendering as a VMware-style info pane. All fields are
// static properties of the definition; live stats (CPU%, rates, etc.)
// come from Snapshot.Domain and the UI's history buffer.
type DomainInfo struct {
	// Identity
	Name        string
	UUID        string
	Title       string
	Description string

	// Hardware
	VCPUs     uint   // current vcpu count
	MaxVCPUs  uint   // max vcpu (for hotplug)
	MemoryKB  uint64 // current (may be ballooned down)
	MaxMemKB  uint64
	CPUMode   string // "host-passthrough", "host-model", "custom"
	CPUModel  string // <model> inside <cpu>, if any

	// OS / boot
	OSType    string   // "hvm", "xen", ...
	OSArch    string   // "x86_64"
	Machine   string   // "pc-q35-9.2"
	Firmware  string   // "BIOS" or "UEFI"
	BootOrder []string // ["hd", "cdrom"]

	// Devices
	Disks    []DiskInfo
	NICs     []NICInfo
	Graphics []GraphicsInfo
}

// DiskInfo describes one disk device from the domain XML.
type DiskInfo struct {
	Target     string // "vda", "sda", "hdc"
	Device     string // "disk", "cdrom", "floppy"
	Bus        string // "virtio", "sata", "scsi", "ide"
	DriverType string // "qcow2", "raw"
	Source     string // source file / dev / network URL
	ReadOnly   bool
	Shareable  bool
}

// NICInfo describes one network interface from the domain XML.
type NICInfo struct {
	MAC        string
	Model      string // "virtio", "e1000", ...
	SourceType string // "bridge", "network", "user", "direct"
	Source     string // bridge name, libvirt network name, etc.
	Target     string // host-side tap device ("vnet0")
}

// GraphicsInfo describes one graphical display channel from the XML.
type GraphicsInfo struct {
	Type   string // "spice", "vnc", "rdp"
	Port   int    // -1 if autoport
	Listen string // usually "127.0.0.1" or "0.0.0.0"
}

// DomainInfo fetches the named domain's live XML and parses it into a
// DomainInfo struct. Returns an error if the domain is missing or the
// XML does not parse; does NOT fail if individual fields are absent —
// those simply remain zero-valued.
func (c *Client) DomainInfo(name string) (DomainInfo, error) {
	var out DomainInfo
	err := c.withDomain(name, func(d *libvirt.Domain) error {
		x, err := d.GetXMLDesc(0)
		if err != nil {
			return fmt.Errorf("GetXMLDesc: %w", err)
		}
		out = ParseDomainInfo(x)
		return nil
	})
	return out, err
}

// ParseDomainInfo extracts structured info from a libvirt domain XML
// string. A malformed document yields a zero DomainInfo rather than
// an error — the renderer then shows mostly blank fields instead of
// crashing, which is kinder to the user.
func ParseDomainInfo(x string) DomainInfo {
	type sourceAddr struct {
		File    string `xml:"file,attr"`
		Dev     string `xml:"dev,attr"`
		Name    string `xml:"name,attr"`
		Bridge  string `xml:"bridge,attr"`
		Network string `xml:"network,attr"`
		Host    string `xml:"host,attr"`
	}
	type diskTag struct {
		Type   string `xml:"type,attr"`
		Device string `xml:"device,attr"`
		Driver struct {
			Name string `xml:"name,attr"`
			Type string `xml:"type,attr"`
		} `xml:"driver"`
		Source sourceAddr `xml:"source"`
		Target struct {
			Dev string `xml:"dev,attr"`
			Bus string `xml:"bus,attr"`
		} `xml:"target"`
		ReadOnly  *struct{} `xml:"readonly"`
		Shareable *struct{} `xml:"shareable"`
	}
	type macTag struct {
		Address string `xml:"address,attr"`
	}
	type modelTag struct {
		Type string `xml:"type,attr"`
	}
	type targetTag struct {
		Dev string `xml:"dev,attr"`
	}
	type ifaceTag struct {
		Type   string     `xml:"type,attr"`
		MAC    macTag     `xml:"mac"`
		Source sourceAddr `xml:"source"`
		Model  modelTag   `xml:"model"`
		Target targetTag  `xml:"target"`
	}
	type graphicsTag struct {
		Type     string `xml:"type,attr"`
		Port     string `xml:"port,attr"`
		AutoPort string `xml:"autoport,attr"`
		Listen   string `xml:"listen,attr"`
	}
	type devicesTag struct {
		Disks    []diskTag     `xml:"disk"`
		Ifaces   []ifaceTag    `xml:"interface"`
		Graphics []graphicsTag `xml:"graphics"`
	}
	type bootTag struct {
		Dev string `xml:"dev,attr"`
	}
	type osTypeTag struct {
		Arch    string `xml:",chardata"`
		ArchAtt string `xml:"arch,attr"`
		Machine string `xml:"machine,attr"`
	}
	type osTag struct {
		Type     osTypeTag `xml:"type"`
		Boot     []bootTag `xml:"boot"`
		Loader   string    `xml:"loader"`
		Firmware string    `xml:"firmware,attr"`
	}
	type cpuModelTag struct {
		Type string `xml:",chardata"`
	}
	type cpuTag struct {
		Mode  string      `xml:"mode,attr"`
		Model cpuModelTag `xml:"model"`
	}
	type vcpuTag struct {
		Current string `xml:"current,attr"`
		Max     uint   `xml:",chardata"`
	}
	type memoryTag struct {
		Unit string `xml:"unit,attr"`
		KB   uint64 `xml:",chardata"`
	}
	type domainTag struct {
		Name          string     `xml:"name"`
		UUID          string     `xml:"uuid"`
		Title         string     `xml:"title"`
		Description   string     `xml:"description"`
		Memory        memoryTag  `xml:"memory"`
		CurrentMemory memoryTag  `xml:"currentMemory"`
		VCPU          vcpuTag    `xml:"vcpu"`
		OS            osTag      `xml:"os"`
		CPU           cpuTag     `xml:"cpu"`
		Devices       devicesTag `xml:"devices"`
	}

	var d domainTag
	if err := xml.Unmarshal([]byte(x), &d); err != nil {
		return DomainInfo{}
	}

	out := DomainInfo{
		Name:        d.Name,
		UUID:        d.UUID,
		Title:       strings.TrimSpace(d.Title),
		Description: strings.TrimSpace(d.Description),
		MaxMemKB:    kbFromMemoryTag(d.Memory),
		MemoryKB:    kbFromMemoryTag(d.CurrentMemory),
		MaxVCPUs:    d.VCPU.Max,
		OSType:      strings.TrimSpace(d.OS.Type.Arch),
		OSArch:      d.OS.Type.ArchAtt,
		Machine:     d.OS.Type.Machine,
		CPUMode:     d.CPU.Mode,
		CPUModel:    strings.TrimSpace(d.CPU.Model.Type),
	}
	if out.MemoryKB == 0 {
		out.MemoryKB = out.MaxMemKB
	}
	// vcpu current attribute is optional; fall back to the max.
	if d.VCPU.Current != "" {
		if v, err := strconv.ParseUint(d.VCPU.Current, 10, 32); err == nil {
			out.VCPUs = uint(v)
		}
	}
	if out.VCPUs == 0 {
		out.VCPUs = out.MaxVCPUs
	}

	for _, b := range d.OS.Boot {
		if b.Dev != "" {
			out.BootOrder = append(out.BootOrder, b.Dev)
		}
	}
	// Firmware detection: the attribute firmware="efi" is the modern
	// form; absent that, presence of a <loader> path that contains
	// "OVMF" or "edk2" is the fallback hint for UEFI.
	switch {
	case d.OS.Firmware == "efi":
		out.Firmware = "UEFI"
	case strings.Contains(strings.ToLower(d.OS.Loader), "ovmf"),
		strings.Contains(strings.ToLower(d.OS.Loader), "edk2"):
		out.Firmware = "UEFI"
	default:
		out.Firmware = "BIOS"
	}

	for _, dk := range d.Devices.Disks {
		device := dk.Device
		if device == "" {
			device = "disk"
		}
		disk := DiskInfo{
			Target:     dk.Target.Dev,
			Device:     device,
			Bus:        dk.Target.Bus,
			DriverType: dk.Driver.Type,
			ReadOnly:   dk.ReadOnly != nil,
			Shareable:  dk.Shareable != nil,
		}
		// Prefer the most informative source attribute.
		switch {
		case dk.Source.File != "":
			disk.Source = dk.Source.File
		case dk.Source.Dev != "":
			disk.Source = dk.Source.Dev
		case dk.Source.Name != "":
			disk.Source = dk.Source.Name
		case dk.Source.Host != "":
			disk.Source = dk.Source.Host
		}
		out.Disks = append(out.Disks, disk)
	}

	for _, i := range d.Devices.Ifaces {
		nic := NICInfo{
			MAC:        i.MAC.Address,
			Model:      i.Model.Type,
			SourceType: i.Type,
			Target:     i.Target.Dev,
		}
		switch {
		case i.Source.Bridge != "":
			nic.Source = i.Source.Bridge
		case i.Source.Network != "":
			nic.Source = i.Source.Network
		case i.Source.Dev != "":
			nic.Source = i.Source.Dev
		}
		out.NICs = append(out.NICs, nic)
	}

	for _, g := range d.Devices.Graphics {
		info := GraphicsInfo{
			Type:   g.Type,
			Listen: g.Listen,
		}
		if g.AutoPort == "yes" {
			info.Port = -1
		} else if g.Port != "" {
			if p, err := strconv.Atoi(g.Port); err == nil {
				info.Port = p
			}
		}
		out.Graphics = append(out.Graphics, info)
	}

	return out
}

// kbFromMemoryTag normalises a libvirt <memory unit='...'> tag into KB
// so callers don't need to worry about the unit attribute.
func kbFromMemoryTag(m struct {
	Unit string `xml:"unit,attr"`
	KB   uint64 `xml:",chardata"`
}) uint64 {
	v := m.KB
	switch strings.ToLower(m.Unit) {
	case "", "kib", "kb", "k":
		return v
	case "mib", "mb", "m":
		return v * 1024
	case "gib", "gb", "g":
		return v * 1024 * 1024
	case "b", "bytes":
		return v / 1024
	}
	return v
}
