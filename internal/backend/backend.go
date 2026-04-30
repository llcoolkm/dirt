// Package backend defines the contract dirt's UI uses to talk to a
// hypervisor. *lv.Client is the canonical implementation; future
// backends (e.g. Proxmox VE's REST API) need only satisfy this
// interface to slot in unchanged.
//
// The data shapes (lv.Domain, lv.HostInfo, lv.Network, …) live in
// internal/lv for now and are referenced here directly. If a second
// backend lands, those types should move to a neutral package — the
// interface here will not have to change.
package backend

import (
	"github.com/llcoolkm/dirt/internal/lv"
)

// Backend is everything dirt's UI asks of a hypervisor connection.
// Methods are grouped by what they touch (host, domain, snapshot,
// network, pool/volume, migration) rather than by libvirt's flat
// API surface.
type Backend interface {
	// Connection identity
	URI() string
	Hostname() string
	Close()

	// Host / sampling
	Snapshot() (*lv.Snapshot, error)
	Host() (lv.HostInfo, error)
	HostStats() (lv.HostStats, error)
	XMLDesc(name string) (string, error)
	DomainInfo(name string) (lv.DomainInfo, error)
	QueryGuestUptime(name string) lv.GuestUptime
	Swap(name string) lv.SwapInfo

	// Domain lifecycle
	Start(name string) error
	Shutdown(name string) error
	Destroy(name string) error
	Reboot(name string) error
	Suspend(name string) error
	Resume(name string) error
	Undefine(name string) error
	UndefineAndDelete(name string) (warnings []string, err error)
	Clone(src, dst string) error

	// Hotplug
	AttachDisk(domain, sourcePath, target string) error
	DetachDisk(domain, target string) error
	AttachNIC(domain, network string) error
	DetachNIC(domain, mac string) error

	// Snapshots
	ListSnapshots(name string) ([]lv.DomainSnapshot, error)
	CreateSnapshot(domain, snapName, description string) error
	RevertSnapshot(domain, snapName string) error
	DeleteSnapshot(domain, snapName string) error

	// Networks
	ListNetworks() ([]lv.Network, error)
	StartNetwork(name string) error
	StopNetwork(name string) error
	ToggleNetworkAutostart(name string) error
	ListDHCPLeases(netName string) ([]lv.DHCPLease, error)

	// Storage pools / volumes
	ListStoragePools() ([]lv.StoragePool, error)
	StartPool(name string) error
	StopPool(name string) error
	ListVolumes(poolName string) ([]lv.StorageVolume, error)
	CreateVolume(poolName, volName string, capacityBytes uint64) error
	DeleteVolume(poolName, volName string) error

	// Migration
	Migrate(name string, opts lv.MigrateOptions) error
	MigrationProgress(name string) (running bool, info lv.MigrateInfo, err error)
	AbortMigration(name string) error
}

// Compile-time assertion that *lv.Client satisfies Backend. This
// catches drift the moment either side changes a signature.
var _ Backend = (*lv.Client)(nil)
