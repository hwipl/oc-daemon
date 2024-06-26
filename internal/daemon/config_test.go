package daemon

import (
	"os"
	"reflect"
	"testing"

	"github.com/telekom-mms/oc-daemon/internal/api"
	"github.com/telekom-mms/oc-daemon/internal/cpd"
	"github.com/telekom-mms/oc-daemon/internal/dnsproxy"
	"github.com/telekom-mms/oc-daemon/internal/execs"
	"github.com/telekom-mms/oc-daemon/internal/ocrunner"
	"github.com/telekom-mms/oc-daemon/internal/splitrt"
	"github.com/telekom-mms/oc-daemon/internal/trafpol"
	"github.com/telekom-mms/tnd/pkg/tnd"
)

// TestConfigString tests String of Config.
func TestConfigString(t *testing.T) {
	// test new config
	c := NewConfig()
	if c.String() == "" {
		t.Errorf("string should not be empty: %s", c.String())
	}

	// test nil
	c = nil
	if c.String() != "null" {
		t.Errorf("string should be null: %s", c.String())
	}
}

// TestConfigValid tests Valid of Config.
func TestConfigValid(t *testing.T) {
	// test invalid
	for _, invalid := range []*Config{
		nil,
		{},
	} {
		want := false
		got := invalid.Valid()

		if got != want {
			t.Errorf("got %t, want %t for %v", got, want, invalid)
		}
	}

	// test valid
	valid := NewConfig()
	want := true
	got := valid.Valid()

	if got != want {
		t.Errorf("got %t, want %t for %v", got, want, valid)
	}
}

// TestConfigLoad tests Load of Config.
func TestConfigLoad(t *testing.T) {
	config := NewConfig()
	config.Config = "does not exist"

	// test invalid path
	err := config.Load()
	if err == nil {
		t.Errorf("got != nil, want nil")
	}

	// test empty config file
	empty, err := os.CreateTemp("", "oc-daemon-config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Remove(empty.Name())
	}()

	config = NewConfig()
	config.Config = empty.Name()
	err = config.Load()
	if err == nil {
		t.Errorf("got != nil, want nil")
	}

	// test valid config file
	// - complete config
	// - partial config with defaults
	for _, content := range []string{
		`{
	"Verbose": true,
	"SocketServer": {
		"SocketFile": "/run/oc-daemon/daemon.sock",
		"SocketOwner": "",
		"SocketGroup": "",
		"SocketPermissions":  "0700",
		"RequestTimeout": 30000000000
	},
	"CPD": {
		"Host": "connectivity-check.ubuntu.com",
		"HTTPTimeout": 5000000000,
		"ProbeCount": 3,
		"ProbeTimer": 300000000000,
		"ProbeTimerDetected": 15000000000
	},
	"DNSProxy": {
		"Address": "127.0.0.1:4253",
		"ListenUDP": true,
		"ListenTCP": true
	},
	"OpenConnect": {
		"OpenConnect": "openconnect",
		"XMLProfile": "/var/lib/oc-daemon/profile.xml",
		"VPNCScript": "/usr/bin/oc-daemon-vpncscript",
		"VPNDevice": "oc-daemon-tun0",
		"PIDFile": "/run/oc-daemon/openconnect.pid",
		"PIDOwner": "",
		"PIDGroup": "",
		"PIDPermissions": "0600"
	},
	"Executables": {
		"IP": "ip",
		"Nft": "nft",
		"Resolvectl": "resolvectl",
		"Sysctl": "sysctl"
	},
	"SplitRouting": {
		"RoutingTable": "42111",
		"RulePriority1": "2111",
		"RulePriority2": "2112",
		"FirewallMark": "42111"
	},
	"TrafficPolicing": {
		"AllowedHosts": ["connectivity-check.ubuntu.com", "detectportal.firefox.com", "www.gstatic.com", "clients3.google.com", "nmcheck.gnome.org"],
		"PortalPorts": [80, 443],
		"ResolveTimeout": 2000000000,
		"ResolveTries": 3,
		"ResolveTriesSleep": 1000000000,
		"ResolveTTL": 300000000000
	},
	"TND": {
		"WaitCheck": 1000000000,
		"HTTPSTimeout": 5000000000,
		"UntrustedTimer": 30000000000,
		"TrustedTimer": 60000000000
	}
}`,
		`{
	"Verbose": true
}`,
	} {

		valid, err := os.CreateTemp("", "oc-daemon-config-test")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Remove(valid.Name())
		}()

		if _, err := valid.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}

		config := NewConfig()
		config.Config = valid.Name()
		if err := config.Load(); err != nil {
			t.Errorf("could not load valid config: %s", err)
		}

		if !config.Valid() {
			t.Errorf("config is not valid")
		}

		want := &Config{
			Config:          valid.Name(),
			Verbose:         true,
			SocketServer:    api.NewConfig(),
			CPD:             cpd.NewConfig(),
			DNSProxy:        dnsproxy.NewConfig(),
			OpenConnect:     ocrunner.NewConfig(),
			Executables:     execs.NewConfig(),
			SplitRouting:    splitrt.NewConfig(),
			TrafficPolicing: trafpol.NewConfig(),
			TND:             tnd.NewConfig(),
		}
		if !reflect.DeepEqual(want.DNSProxy, config.DNSProxy) {
			t.Errorf("got %v, want %v", config.DNSProxy, want.DNSProxy)
		}
		if !reflect.DeepEqual(want.OpenConnect, config.OpenConnect) {
			t.Errorf("got %v, want %v", config.OpenConnect, want.OpenConnect)
		}
		if !reflect.DeepEqual(want.Executables, config.Executables) {
			t.Errorf("got %v, want %v", config.Executables, want.Executables)
		}
		if !reflect.DeepEqual(want.SplitRouting, config.SplitRouting) {
			t.Errorf("got %v, want %v", config.SplitRouting, want.SplitRouting)
		}
		if !reflect.DeepEqual(want.TrafficPolicing, config.TrafficPolicing) {
			t.Errorf("got %v, want %v", config.TrafficPolicing, want.TrafficPolicing)
		}
		if !reflect.DeepEqual(want.TND, config.TND) {
			t.Errorf("got %v, want %v", config.TND, want.TND)
		}
		if !reflect.DeepEqual(want, config) {
			t.Errorf("got %v, want %v", config, want)
		}
	}
}

// TestNewConfig tests NewConfig.
func TestNewConfig(t *testing.T) {
	want := &Config{
		Config:          "/var/lib/oc-daemon/oc-daemon.json",
		Verbose:         false,
		SocketServer:    api.NewConfig(),
		CPD:             cpd.NewConfig(),
		DNSProxy:        dnsproxy.NewConfig(),
		OpenConnect:     ocrunner.NewConfig(),
		Executables:     execs.NewConfig(),
		SplitRouting:    splitrt.NewConfig(),
		TrafficPolicing: trafpol.NewConfig(),
		TND:             tnd.NewConfig(),
	}
	got := NewConfig()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
