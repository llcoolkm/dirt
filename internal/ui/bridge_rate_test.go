package ui

import (
	"testing"
	"time"

	"github.com/llcoolkm/dirt/internal/lv"
)

func TestUpdateBridgeStatsFirstSampleNoRate(t *testing.T) {
	m := &Model{}
	m.updateBridgeStats([]lv.BridgeStats{
		{Name: "br0", OK: true, RxBytes: 1000, TxBytes: 500},
	})
	r := m.bridgeRates["br0"]
	if r.available {
		t.Error("first sample should not produce a rate (no prev to diff against)")
	}
}

func TestUpdateBridgeStatsRateFromTwoSamples(t *testing.T) {
	m := &Model{}
	// First sample.
	m.updateBridgeStats([]lv.BridgeStats{
		{Name: "br0", OK: true, RxBytes: 1000, TxBytes: 500},
	})
	// Backdate the prevAt so the dt computed by updateBridgeStats is sane.
	r := m.bridgeRates["br0"]
	r.prevAt = time.Now().Add(-1 * time.Second)
	m.bridgeRates["br0"] = r
	// Second sample arrives one second later.
	m.updateBridgeStats([]lv.BridgeStats{
		{Name: "br0", OK: true, RxBytes: 2000, TxBytes: 1500},
	})
	got := m.bridgeRates["br0"]
	if !got.available {
		t.Fatal("rate should be available after two samples ≥ 100ms apart")
	}
	if got.rxBps < 900 || got.rxBps > 1100 {
		t.Errorf("rxBps=%.1f, want ~1000", got.rxBps)
	}
	if got.txBps < 900 || got.txBps > 1100 {
		t.Errorf("txBps=%.1f, want ~1000", got.txBps)
	}
}

func TestUpdateBridgeStatsResetOnUnavailable(t *testing.T) {
	m := &Model{bridgeRates: map[string]bridgeRate{
		"br0": {available: true, rxBps: 100},
	}}
	m.updateBridgeStats([]lv.BridgeStats{{Name: "br0", OK: false}})
	if _, has := m.bridgeRates["br0"]; has {
		t.Error("OK=false sample should drop the cached rate")
	}
}

func TestUpdateBridgeStatsRejectsCounterWrap(t *testing.T) {
	m := &Model{}
	m.updateBridgeStats([]lv.BridgeStats{
		{Name: "br0", OK: true, RxBytes: 5000, TxBytes: 5000},
	})
	r := m.bridgeRates["br0"]
	r.prevAt = time.Now().Add(-1 * time.Second)
	m.bridgeRates["br0"] = r
	// Counter goes BACKWARDS — interface reset.
	m.updateBridgeStats([]lv.BridgeStats{
		{Name: "br0", OK: true, RxBytes: 100, TxBytes: 100},
	})
	got := m.bridgeRates["br0"]
	if got.available {
		t.Error("counter wrap should not produce a rate")
	}
}
