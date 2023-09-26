package vpnsetup

import (
	"context"
	"net"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/telekom-mms/oc-daemon/internal/dnsproxy"
	"github.com/telekom-mms/oc-daemon/internal/execs"
	"github.com/telekom-mms/oc-daemon/internal/splitrt"
	"github.com/telekom-mms/oc-daemon/pkg/vpnconfig"
)

// command types
const (
	commandSetup uint8 = iota
	commandTeardown
)

// command is a VPNSetup command
type command struct {
	cmd     uint8
	vpnconf *vpnconfig.Config
}

// Event types
const (
	EventSetupOK uint8 = iota
	EventTeardownOK
)

// Event is a VPNSetup event
type Event struct {
	Type uint8
}

// VPNSetup sets up the configuration of the vpn tunnel that belongs to the
// current VPN connection
type VPNSetup struct {
	splitrt     *splitrt.SplitRouting
	splitrtConf *splitrt.Config

	dnsProxy     *dnsproxy.Proxy
	dnsProxyConf *dnsproxy.Config

	cmds   chan *command
	events chan *Event
	done   chan struct{}
}

// sendEvents sends the event
func (v *VPNSetup) sendEvent(event *Event) {
	select {
	case v.events <- event:
	case <-v.done:
	}
}

// setupVPNDevice sets up the vpn device with config
func setupVPNDevice(c *vpnconfig.Config) {
	ctx := context.TODO()

	// set mtu on device
	mtu := strconv.Itoa(c.Device.MTU)
	if err := execs.RunIPLink(ctx, "set", c.Device.Name, "mtu", mtu); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device": c.Device.Name,
			"mtu":    mtu,
		}).Error("Daemon could not set mtu on device")
		return
	}

	// set device up
	if err := execs.RunIPLink(ctx, "set", c.Device.Name, "up"); err != nil {
		log.WithError(err).WithField("device", c.Device.Name).
			Error("Daemon could not set device up")
		return
	}

	// set ipv4 and ipv6 addresses on device
	setupIP := func(ip net.IP, mask net.IPMask) {
		ipnet := &net.IPNet{
			IP:   ip,
			Mask: mask,
		}
		dev := c.Device.Name
		addr := ipnet.String()
		if err := execs.RunIPAddress(ctx, "add", addr, "dev", dev); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"device": dev,
				"ip":     addr,
			}).Error("Daemon could not set ip on device")
			return
		}

	}
	if len(c.IPv4.Address) > 0 {
		setupIP(c.IPv4.Address, c.IPv4.Netmask)
	}
	if len(c.IPv6.Address) > 0 {
		setupIP(c.IPv6.Address, c.IPv6.Netmask)
	}
}

// teardownVPNDevice tears down the configured vpn device
func teardownVPNDevice(c *vpnconfig.Config) {
	ctx := context.TODO()

	// set device down
	if err := execs.RunIPLink(ctx, "set", c.Device.Name, "down"); err != nil {
		log.WithError(err).WithField("device", c.Device.Name).
			Error("Daemon could not set device down")
		return
	}

}

// setupRouting sets up routing using config
func (v *VPNSetup) setupRouting(vpnconf *vpnconfig.Config) {
	if v.splitrt != nil {
		return
	}
	v.splitrt = splitrt.NewSplitRouting(v.splitrtConf, vpnconf)
	v.splitrt.Start()
}

// teardownRouting tears down the routing configuration
func (v *VPNSetup) teardownRouting() {
	if v.splitrt == nil {
		return
	}
	v.splitrt.Stop()
	v.splitrt = nil
}

// setupDNS sets up DNS using config
func (v *VPNSetup) setupDNS(config *vpnconfig.Config) {
	ctx := context.TODO()

	// configure dns proxy

	// set remotes
	remotes := config.DNS.Remotes()
	v.dnsProxy.SetRemotes(remotes)

	// set watches
	excludes := config.Split.DNSExcludes()
	log.WithField("excludes", excludes).Debug("Daemon setting DNS Split Excludes")
	v.dnsProxy.SetWatches(excludes)

	// update dns configuration of host

	// set dns server for device
	device := config.Device.Name
	if err := execs.RunResolvectl(ctx, "dns", device, v.dnsProxyConf.Address); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device": device,
			"server": v.dnsProxyConf.Address,
		}).Error("VPNSetup error setting dns server")
	}

	// set domains for device
	// this includes "~." to use this device for all domains
	if err := execs.RunResolvectl(ctx, "domain", device, config.DNS.DefaultDomain, "~."); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device": device,
			"domain": config.DNS.DefaultDomain,
		}).Error("VPNSetup error setting dns domains")
	}

	// set default route for device
	if err := execs.RunResolvectl(ctx, "default-route", device, "yes"); err != nil {
		log.WithError(err).WithField("device", device).
			Error("VPNSetup error setting dns default route")
	}

	// flush dns caches
	if err := execs.RunResolvectl(ctx, "flush-caches"); err != nil {
		log.WithError(err).Error("VPNSetup error flushing dns caches during setup")
	}

	// reset learnt server features
	if err := execs.RunResolvectl(ctx, "reset-server-features"); err != nil {
		log.WithError(err).Error("VPNSetup error resetting server features during setup")
	}
}

// teardownDNS tears down the DNS configuration
func (v *VPNSetup) teardownDNS(vpnconf *vpnconfig.Config) {
	ctx := context.TODO()

	// update dns proxy configuration

	// reset remotes
	remotes := map[string][]string{}
	v.dnsProxy.SetRemotes(remotes)

	// reset watches
	v.dnsProxy.SetWatches([]string{})

	// update dns configuration of host

	// undo device dns configuration
	if err := execs.RunResolvectl(ctx, "revert", vpnconf.Device.Name); err != nil {
		log.WithError(err).WithField("device", vpnconf.Device.Name).
			Error("VPNSetup error reverting dns configuration")
	}

	// flush dns caches
	if err := execs.RunResolvectl(ctx, "flush-caches"); err != nil {
		log.WithError(err).Error("VPNSetup error flushing dns caches during teardown")
	}

	// reset learnt server features
	if err := execs.RunResolvectl(ctx, "reset-server-features"); err != nil {
		log.WithError(err).Error("VPNSetup error resetting server features during teardown")
	}
}

// setup sets up the vpn configuration
func (v *VPNSetup) setup(vpnconf *vpnconfig.Config) {
	setupVPNDevice(vpnconf)
	v.setupRouting(vpnconf)
	v.setupDNS(vpnconf)

	v.sendEvent(&Event{EventSetupOK})
}

// teardown tears down the vpn configuration
func (v *VPNSetup) teardown(vpnconf *vpnconfig.Config) {
	teardownVPNDevice(vpnconf)
	v.teardownRouting()
	v.teardownDNS(vpnconf)

	v.sendEvent(&Event{EventTeardownOK})
}

// handleCommand handles a command
func (v *VPNSetup) handleCommand(c *command) {
	switch c.cmd {
	case commandSetup:
		v.setup(c.vpnconf)
	case commandTeardown:
		v.teardown(c.vpnconf)
	}
}

// handleDNSReport handles a DNS report
func (v *VPNSetup) handleDNSReport(r *dnsproxy.Report) {
	log.WithField("report", r).Debug("Daemon handling DNS report")

	if v.splitrt == nil {
		return
	}

	// forward report to split routing
	select {
	case v.splitrt.DNSReports() <- r:
	case <-v.done:
	}
}

// start starts the VPN setup
func (v *VPNSetup) start() {
	defer close(v.events)

	// start DNS-Proxy
	v.dnsProxy.Start()
	defer v.dnsProxy.Stop()

	for {
		select {
		case c := <-v.cmds:
			v.handleCommand(c)
		case r := <-v.dnsProxy.Reports():
			v.handleDNSReport(r)
		case <-v.done:
			return
		}
	}
}

// Start starts the VPN setup
func (v *VPNSetup) Start() {
	go v.start()
}

// Stop stops the VPN setup
func (v *VPNSetup) Stop() {
	close(v.done)
	for range v.events {
		// wait until channel closed
	}
}

// Setup sets the VPN config up
func (v *VPNSetup) Setup(vpnconfig *vpnconfig.Config) {
	v.cmds <- &command{
		cmd:     commandSetup,
		vpnconf: vpnconfig,
	}
}

// Teardown tears the VPN config down
func (v *VPNSetup) Teardown(vpnconfig *vpnconfig.Config) {
	v.cmds <- &command{
		cmd:     commandTeardown,
		vpnconf: vpnconfig,
	}
}

// Events returns the events channel
func (v *VPNSetup) Events() chan *Event {
	return v.events
}

// NewVPNSetup returns a new VPNSetup
func NewVPNSetup(
	dnsProxyConfig *dnsproxy.Config,
	splitrtConfig *splitrt.Config,
) *VPNSetup {
	return &VPNSetup{
		dnsProxy:     dnsproxy.NewProxy(dnsProxyConfig),
		dnsProxyConf: dnsProxyConfig,
		splitrtConf:  splitrtConfig,

		cmds:   make(chan *command),
		events: make(chan *Event),
		done:   make(chan struct{}),
	}
}

// Cleanup cleans up the configuration after a failed shutdown
func Cleanup(vpnDevice string, splitrtConfig *splitrt.Config) {
	ctx := context.TODO()

	// dns, device, split routing
	if err := execs.RunResolvectl(ctx, "revert", vpnDevice); err == nil {
		log.WithField("device", vpnDevice).
			Warn("VPNSetup cleaned up dns config")
	}
	if err := execs.RunIPLink(ctx, "delete", vpnDevice); err == nil {
		log.WithField("device", vpnDevice).
			Warn("VPNSetup cleaned up vpn device")
	}
	splitrt.Cleanup(splitrtConfig)
}
