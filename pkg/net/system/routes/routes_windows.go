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

package routes

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net/netip"
	"strings"

	"github.com/webmeshproj/webmesh/pkg/util"
)

// GetDefaultGateway returns the default gateway of the current system.
func GetDefaultGateway(ctx context.Context) (netip.Addr, error) {
	return defaultGatewayIPConfig(ctx)
}

// SetDefaultIPv4Gateway sets the default IPv4 gateway for the current system.
func SetDefaultIPv4Gateway(ctx context.Context, gateway netip.Addr) error {
	return errors.New("not implemented")
}

// SetDefaultIPv6Gateway sets the default IPv6 gateway for the current system.
func SetDefaultIPv6Gateway(ctx context.Context, gateway netip.Addr) error {
	return errors.New("not implemented")
}

// Add adds a route to the interface with the given name.
func Add(ctx context.Context, ifaceName string, addr netip.Prefix) error {
	family := "ipv4"
	if addr.Addr().Is6() {
		family = "ipv6"
	}
	out, err := util.ExecOutput(ctx, "netsh", "interface", family, "add", "route", addr.String(), ifaceName, "metric=0", "store=active")
	if err != nil {
		if strings.Contains(string(out), "already in table") || strings.Contains(string(out), "exists") {
			return ErrRouteExists
		}
		return err
	}
	return nil
}

// Remove removes a route from the interface with the given name.
func Remove(ctx context.Context, ifaceName string, addr netip.Prefix) error {
	family := "ipv4"
	if addr.Addr().Is6() {
		family = "ipv6"
	}
	err := util.Exec(ctx, "netsh", "interface", family, "delete", "route", addr.String(), ifaceName, "metric=0", "store=active")
	if err != nil {
		return err
	}
	return nil
}

func defaultGatewayIPConfig(ctx context.Context) (netip.Addr, error) {
	out, err := util.ExecOutput(ctx, "ipconfig")
	if err != nil {
		return netip.Addr{}, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Default Gateway") {
			fields := strings.Fields(line)
			return netip.ParseAddr(fields[len(fields)-1])
		}
	}
	return netip.Addr{}, nil
}
