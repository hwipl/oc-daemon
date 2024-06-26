package trafpol

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/telekom-mms/oc-daemon/internal/cpd"
	"github.com/telekom-mms/oc-daemon/internal/devmon"
	"github.com/telekom-mms/oc-daemon/internal/execs"
	"github.com/vishvananda/netlink"
)

// TestTrafPolHandleDeviceUpdate tests handleDeviceUpdate of TrafPol.
func TestTrafPolHandleDeviceUpdate(_ *testing.T) {
	tp := NewTrafPol(NewConfig())
	ctx := context.Background()

	// test adding
	update := &devmon.Update{
		Add: true,
	}
	tp.handleDeviceUpdate(ctx, update)

	// test removing
	update.Add = false
	tp.handleDeviceUpdate(ctx, update)
}

// TestTrafPolHandleDNSUpdate tests handleDNSUpdate of TrafPol.
func TestTrafPolHandleDNSUpdate(_ *testing.T) {
	tp := NewTrafPol(NewConfig())

	tp.allowHosts.Start()
	defer tp.allowHosts.Stop()
	tp.cpd.Start()
	defer tp.cpd.Stop()

	tp.handleDNSUpdate()
}

// TestTrafPolHandleCPDReport tests handleCPDReport of TrafPol.
func TestTrafPolHandleCPDReport(t *testing.T) {
	tp := NewTrafPol(NewConfig())
	ctx := context.Background()

	tp.allowHosts.Start()
	defer tp.allowHosts.Stop()

	var nftMutex sync.Mutex
	nftCmds := []string{}
	oldRunCmd := execs.RunCmd
	execs.RunCmd = func(_ context.Context, _ string, s string, _ ...string) ([]byte, []byte, error) {
		nftMutex.Lock()
		defer nftMutex.Unlock()
		nftCmds = append(nftCmds, s)
		return nil, nil, nil
	}
	defer func() { execs.RunCmd = oldRunCmd }()

	getNftCmds := func() []string {
		nftMutex.Lock()
		defer nftMutex.Unlock()
		return append(nftCmds[:0:0], nftCmds...)
	}

	// test not detected
	report := &cpd.Report{}
	tp.handleCPDReport(ctx, report)

	want := []string{}
	got := getNftCmds()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// test detected
	report.Detected = true
	tp.handleCPDReport(ctx, report)

	want = []string{
		"add element inet oc-daemon-filter allowports { 80, 443 }",
	}
	got = getNftCmds()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// test not detected any more
	report.Detected = false
	tp.handleCPDReport(ctx, report)

	want = []string{
		"add element inet oc-daemon-filter allowports { 80, 443 }",
		"delete element inet oc-daemon-filter allowports { 80, 443 }",
	}
	got = getNftCmds()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestTrafPolStartEvents tests start of TrafPol, events.
func TestTrafPolStartEvents(t *testing.T) {
	// set dummy low level function for devmon
	oldRegisterLinkUpdates := devmon.RegisterLinkUpdates
	devmon.RegisterLinkUpdates = func(*devmon.DevMon) (chan netlink.LinkUpdate, error) {
		return nil, nil
	}
	defer func() { devmon.RegisterLinkUpdates = oldRegisterLinkUpdates }()

	tp := NewTrafPol(NewConfig())
	if err := tp.Start(); err != nil {
		t.Fatal(err)
	}
	tp.devmon.Updates() <- &devmon.Update{Type: "device"}
	tp.dnsmon.Updates() <- struct{}{}
	tp.cpd.Results() <- &cpd.Report{}
	tp.Stop()
}

// TestTrafPolStartStop tests Start and Stop of TrafPol.
func TestTrafPolStartStop(t *testing.T) {
	// set dummy low level function for devmon
	oldRegisterLinkUpdates := devmon.RegisterLinkUpdates
	devmon.RegisterLinkUpdates = func(*devmon.DevMon) (chan netlink.LinkUpdate, error) {
		return nil, nil
	}
	defer func() { devmon.RegisterLinkUpdates = oldRegisterLinkUpdates }()

	tp := NewTrafPol(NewConfig())
	if err := tp.Start(); err != nil {
		t.Fatal(err)
	}
	tp.Stop()
}

// TestNewTrafPol tests NewTrafPol.
func TestNewTrafPol(t *testing.T) {
	tp := NewTrafPol(NewConfig())
	if tp.devmon == nil ||
		tp.dnsmon == nil ||
		tp.cpd == nil ||
		tp.allowDevs == nil ||
		tp.allowHosts == nil ||
		tp.loopDone == nil ||
		tp.done == nil {

		t.Errorf("got nil, want != nil")
	}
}

// TestCleanup tests Cleanup.
func TestCleanup(t *testing.T) {
	want := []string{
		"delete table inet oc-daemon-filter",
	}
	got := []string{}
	execs.RunCmd = func(_ context.Context, _ string, s string, _ ...string) ([]byte, []byte, error) {
		got = append(got, s)
		return nil, nil, nil
	}
	Cleanup(context.Background())
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
