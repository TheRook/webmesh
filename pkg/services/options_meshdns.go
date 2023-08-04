/*
Copyright 2023 Avi Zimmerman <avi.zimmerman@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"errors"
	"flag"
	"os"
	"strings"
	"time"

	"github.com/webmeshproj/webmesh/pkg/util"
)

const (
	MeshDNSEnabledEnvVar           = "SERVICES_MESH_DNS_ENABLED"
	MeshDNSListenUDPEnvVar         = "SERVICES_MESH_DNS_LISTEN_UDP"
	MeshDNSListenTCPEnvVar         = "SERVICES_MESH_DNS_LISTEN_TCP"
	MeshDNSTSIGKeyEnvVar           = "SERVICES_MESH_DNS_TSIG_KEY"
	MeshDNSReusePortEnvVar         = "SERVICES_MESH_DNS_REUSE_PORT"
	MeshDNSCompressionEnvVar       = "SERVICES_MESH_DNS_COMPRESSION"
	MeshDNSRequestTimeoutEnvVar    = "SERVICES_MESH_DNS_REQUEST_TIMEOUT"
	MeshDNSForwardersEnvVar        = "SERVICES_MESH_DNS_FORWARDERS"
	MeshDNSDisableForwardingEnvVar = "SERVICES_MESH_DNS_DISABLE_FORWARDING"
	MeshDNSCacheSizeEnvVar         = "SERVICES_MESH_DNS_CACHE_SIZE"
)

// MeshDNSOptions are the mesh DNS options.
type MeshDNSOptions struct {
	// Enabled enables mesh DNS.
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty" toml:"enabled,omitempty"`
	// ListenUDP is the UDP address to listen on.
	ListenUDP string `yaml:"listen-udp,omitempty" json:"listen-udp,omitempty" toml:"listen-udp,omitempty"`
	// ListenTCP is the address to listen on for TCP DNS requests.
	ListenTCP string `json:"listen-tcp,omitempty" yaml:"listen-tcp,omitempty" toml:"listen-tcp,omitempty"`
	// ReusePort sets the number of listeners to start on each port.
	// This is only supported on Linux.
	ReusePort int `json:"reuse-port,omitempty" yaml:"reuse-port,omitempty" toml:"reuse-port,omitempty"`
	// EnableCompression is true if DNS compression should be enabled.
	EnableCompression bool `json:"compression,omitempty" yaml:"compression,omitempty" toml:"compression,omitempty"`
	// RequestTimeout is the timeout for DNS requests.
	RequestTimeout time.Duration `json:"request-timeout,omitempty" yaml:"request-timeout,omitempty" toml:"request-timeout,omitempty"`
	// Forwarders are the DNS forwarders to use. If empty, the system DNS servers will be used.
	Forwarders []string `json:"forwarders,omitempty" yaml:"forwarders,omitempty" toml:"forwarders,omitempty"`
	// DisableForwarding disables forwarding requests to the configured forwarders.
	DisableForwarding bool `json:"disable-forwarding,omitempty" yaml:"disable-forwarding,omitempty" toml:"disable-forwarding,omitempty"`
	// CacheSize is the size of the remote DNS cache.
	CacheSize int `json:"cache-size,omitempty" yaml:"cache-size,omitempty" toml:"cache-size,omitempty"`
}

// NewMeshDNSOptions creates a new set of mesh DNS options.
func NewMeshDNSOptions() *MeshDNSOptions {
	return &MeshDNSOptions{
		Enabled:           false,
		ListenUDP:         ":5353",
		ListenTCP:         ":5353",
		EnableCompression: true,
		RequestTimeout:    time.Second * 5,
	}
}

// BindFlags binds the flags for the mesh DNS options.
func (o *MeshDNSOptions) BindFlags(fs *flag.FlagSet, prefix ...string) {
	var p string
	if len(prefix) > 0 {
		p = strings.Join(prefix, ".") + "."
	}
	fs.BoolVar(&o.Enabled, p+"services.mesh-dns.enabled", util.GetEnvDefault(MeshDNSEnabledEnvVar, "false") == "true",
		"Enable mesh DNS.")
	fs.StringVar(&o.ListenUDP, p+"services.mesh-dns.listen-udp", util.GetEnvDefault(MeshDNSListenUDPEnvVar, ":5353"),
		"UDP address to listen on for DNS requests.")
	fs.StringVar(&o.ListenTCP, p+"services.mesh-dns.listen-tcp", util.GetEnvDefault(MeshDNSListenTCPEnvVar, ":5353"),
		"TCP address to listen on for DNS requests.")
	fs.IntVar(&o.ReusePort, p+"services.mesh-dns.reuse-port", util.GetEnvIntDefault(MeshDNSReusePortEnvVar, 0),
		"Enable SO_REUSEPORT for mesh DNS.")
	fs.BoolVar(&o.EnableCompression, p+"services.mesh-dns.enable-compression", util.GetEnvDefault(MeshDNSCompressionEnvVar, "true") == "true",
		"Enable DNS compression for mesh DNS.")
	fs.DurationVar(&o.RequestTimeout, p+"services.mesh-dns.request-timeout", util.GetEnvDurationDefault(MeshDNSRequestTimeoutEnvVar, 5*time.Second),
		"Timeout for mesh DNS requests.")
	fs.Func(p+"services.mesh-dns.forwarders", "DNS forwarders to use for mesh DNS. If empty, the system DNS servers will be used.", func(s string) error {
		o.Forwarders = strings.Split(s, ",")
		return nil
	})
	fs.BoolVar(&o.DisableForwarding, p+"services.mesh-dns.disable-forwarding", util.GetEnvDefault(MeshDNSDisableForwardingEnvVar, "false") == "true",
		"Disable forwarding requests to any configured forwarders.")
	fs.IntVar(&o.CacheSize, p+"services.mesh-dns.cache-size", util.GetEnvIntDefault(MeshDNSCacheSizeEnvVar, 0),
		"Size of the remote DNS cache. Defaults to 0 (disabled).")
}

// Validate validates the mesh DNS options.
func (o *MeshDNSOptions) Validate() error {
	if o == nil {
		return nil
	}
	if o.Enabled {
		if o.ListenTCP == "" && o.ListenUDP == "" {
			return errors.New("must specify a TCP or UDP address for mesh DNS")
		}
		if len(o.Forwarders) == 0 && !o.DisableForwarding {
			// Check the environment
			envval := os.Getenv(MeshDNSForwardersEnvVar)
			if envval != "" {
				o.Forwarders = strings.Split(envval, ",")
			}
		}
	}
	return nil
}
