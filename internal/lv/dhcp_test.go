package lv

import "testing"

const testNetXML = `<network>
  <name>default</name>
  <forward mode='nat'/>
  <bridge name='virbr0'/>
  <ip address='192.168.122.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.122.100' end='192.168.122.254'/>
      <host mac='52:54:00:AA:BB:CC' name='web-01' ip='192.168.122.10'/>
      <host mac='52:54:00:11:22:33' name='db-01' ip='192.168.122.11'/>
    </dhcp>
  </ip>
</network>`

func TestParseDHCPReservations(t *testing.T) {
	got := parseDHCPReservations(testNetXML)
	if len(got) != 2 {
		t.Fatalf("got %d reservations, want 2", len(got))
	}
	if got[0].MAC != "52:54:00:AA:BB:CC" || got[0].Name != "web-01" || got[0].IP != "192.168.122.10" {
		t.Errorf("first reservation = %+v", got[0])
	}
	if got[1].Name != "db-01" {
		t.Errorf("second reservation = %+v", got[1])
	}
}

func TestParseDHCPReservationsNoDHCP(t *testing.T) {
	if got := parseDHCPReservations(`<network><name>isolated</name></network>`); len(got) != 0 {
		t.Errorf("expected no reservations, got %+v", got)
	}
	if got := parseDHCPReservations(`not xml`); got != nil {
		t.Errorf("expected nil for invalid XML, got %+v", got)
	}
}
