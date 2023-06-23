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

package util

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

func ifaceNetwork(ifaceName string, forAddr netip.Addr, ipv6 bool) (netip.Prefix, error) {
	out, err := ExecOutput(context.Background(), "ifconfig", ifaceName)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("ifconfig %s: %w: %s", ifaceName, err, out)
	}
	strPrefix := "inet"
	if ipv6 {
		strPrefix = "inet6"
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, strPrefix) {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			addr, prefix := fields[1], fields[3]
			if addr != forAddr.String() {
				continue
			}
			ip, err := netip.ParseAddr(addr)
			if err != nil {
				return netip.Prefix{}, fmt.Errorf("parse %s: %w", addr, err)
			}
			if ipv6 {
				// We have a raw prefixlen in the field
				bits, err := strconv.Atoi(prefix)
				if err != nil {
					return netip.Prefix{}, fmt.Errorf("parse %s: %w", prefix, err)
				}
				return netip.PrefixFrom(ip, bits), nil
			}
			// We have a hex prefix in the field
			bits, err := strconv.ParseUint(prefix, 16, 32)
			if err != nil {
				return netip.Prefix{}, fmt.Errorf("parse %s: %w", prefix, err)
			}
			return netip.PrefixFrom(ip, int(bits)), nil
		}
	}
	return netip.Prefix{}, fmt.Errorf("no %s address found for %s", forAddr, ifaceName)
}
