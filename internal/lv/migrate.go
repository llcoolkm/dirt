package lv

import (
	"fmt"
	"time"

	"libvirt.org/go/libvirt"
)

// MigrateOptions controls how Client.Migrate runs.
type MigrateOptions struct {
	// DestURI is the libvirt URI of the destination daemon AS SEEN BY
	// THE SOURCE. Typically the same qemu+ssh://... you'd pass to virsh.
	DestURI string

	// NewName renames the domain on the destination. Empty = keep name.
	NewName string

	// CopyStorage enables non-shared-disk migration. When false (default),
	// libvirt assumes shared storage (NFS/iSCSI/etc).
	CopyStorage bool

	// BandwidthMiBps caps the migration transfer rate. 0 = unlimited.
	BandwidthMiBps uint64

	// MaxDowntimeMs is the target cutover downtime in milliseconds.
	// 0 = libvirt default (~30ms).
	MaxDowntimeMs uint64
}

// Migrate kicks off a live migration of the named domain to DestURI.
// Runs synchronously (returns when the migration finishes or errors).
//
// Intended to be called from a goroutine; progress must be polled via
// MigrationProgress. AbortMigration cancels a running migration.
func (c *Client) Migrate(name string, opts MigrateOptions) error {
	return c.withDomain(name, func(d *libvirt.Domain) error {
		flags := libvirt.MIGRATE_LIVE |
			libvirt.MIGRATE_PERSIST_DEST |
			libvirt.MIGRATE_UNDEFINE_SOURCE |
			libvirt.MIGRATE_AUTO_CONVERGE

		if opts.CopyStorage {
			flags |= libvirt.MIGRATE_NON_SHARED_DISK
		}

		// Optional downtime tuning — runs on the source domain before
		// migration starts. Best-effort; ignore errors.
		if opts.MaxDowntimeMs > 0 {
			_ = d.MigrateSetMaxDowntime(opts.MaxDowntimeMs, 0)
		}

		params := &libvirt.DomainMigrateParameters{}
		if opts.NewName != "" {
			params.DestName = opts.NewName
			params.DestNameSet = true
		}
		if opts.BandwidthMiBps > 0 {
			params.Bandwidth = opts.BandwidthMiBps
			params.BandwidthSet = true
		}

		return d.MigrateToURI3(opts.DestURI, params, flags)
	})
}

// MigrationProgress polls the live job info on the named source domain.
// Returns (true, info, nil) while a job is running, (false, zero, nil)
// when no job is active, or (false, zero, err) on error.
func (c *Client) MigrationProgress(name string) (running bool, info MigrateInfo, err error) {
	err = c.withDomain(name, func(d *libvirt.Domain) error {
		stats, e := d.GetJobStats(0)
		if e != nil {
			return e
		}
		switch stats.Type {
		case libvirt.DOMAIN_JOB_NONE, libvirt.DOMAIN_JOB_COMPLETED,
			libvirt.DOMAIN_JOB_FAILED, libvirt.DOMAIN_JOB_CANCELLED:
			return nil
		}
		running = true
		info.Elapsed = time.Duration(stats.TimeElapsed) * time.Millisecond
		if stats.DataTotalSet {
			info.DataTotal = stats.DataTotal
		}
		if stats.DataProcessedSet {
			info.DataProcessed = stats.DataProcessed
		}
		if stats.DataRemainingSet {
			info.DataRemaining = stats.DataRemaining
		}
		if stats.MemTotalSet {
			info.MemTotal = stats.MemTotal
		}
		if stats.MemProcessedSet {
			info.MemProcessed = stats.MemProcessed
		}
		if stats.MemRemainingSet {
			info.MemRemaining = stats.MemRemaining
		}
		if stats.DiskTotalSet {
			info.DiskTotal = stats.DiskTotal
		}
		if stats.DiskProcessedSet {
			info.DiskProcessed = stats.DiskProcessed
		}
		if stats.DiskRemainingSet {
			info.DiskRemaining = stats.DiskRemaining
		}
		if stats.MemIterationSet {
			info.Iteration = stats.MemIteration
		}
		return nil
	})
	if err != nil {
		return false, MigrateInfo{}, fmt.Errorf("job stats %s: %w", name, err)
	}
	return running, info, nil
}

// AbortMigration cancels a running migration (or any other running job).
func (c *Client) AbortMigration(name string) error {
	return c.withDomain(name, func(d *libvirt.Domain) error {
		return d.AbortJob()
	})
}

// MigrateInfo is a snapshot of a running migration's progress.
type MigrateInfo struct {
	Elapsed       time.Duration
	DataTotal     uint64
	DataProcessed uint64
	DataRemaining uint64
	MemTotal      uint64
	MemProcessed  uint64
	MemRemaining  uint64
	DiskTotal     uint64
	DiskProcessed uint64
	DiskRemaining uint64
	Iteration     uint64 // memory iteration — high value = VM is dirtying pages faster than we transfer
}
