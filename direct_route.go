package main

import "net/netip"

const (
	routeDirect  = "direct"
	routeTunnel  = "tunnel"
	routeUnknown = "unknown"
)

type DirectRouteCheck struct {
	Version int    `json:"version"`
	State   string `json:"state"`
	Device  string `json:"device"`
}

type routeInterface struct {
	state string
	name  string
}

func (a *App) CheckDirectRoute(version int) []DirectRouteCheck {
	lookup := newRouteLookup()
	switch version {
	case 4:
		return []DirectRouteCheck{inspectRoute(4, routeSamples(cfIPv4CIDRs), lookup)}
	case 6:
		return []DirectRouteCheck{inspectRoute(6, routeSamples(cfIPv6CIDRs), lookup)}
	default:
		return []DirectRouteCheck{
			inspectRoute(4, routeSamples(cfIPv4CIDRs), lookup),
			inspectRoute(6, routeSamples(cfIPv6CIDRs), lookup),
		}
	}
}

func (a *App) CheckSpeedTestRoute(regionCodes []string, count int) []DirectRouteCheck {
	a.mu.Lock()
	targets := a.lastScanResults
	a.mu.Unlock()
	if len(regionCodes) > 0 {
		regions := normalizedRegions(regionCodes)
		if len(regions) == 0 || count < 1 {
			return []DirectRouteCheck{{State: routeUnknown}}
		}
		targets = newSpeedTester(targets, regions, count, 0).targets()
	}
	var v4, v6 []netip.Addr
	for _, target := range targets {
		ip, err := netip.ParseAddr(target.IP)
		if err != nil {
			continue
		}
		if ip.Is4() {
			v4 = append(v4, ip)
		} else {
			v6 = append(v6, ip)
		}
	}
	lookup := newRouteLookup()
	checks := make([]DirectRouteCheck, 0, 2)
	if len(v4) != 0 {
		checks = append(checks, inspectRoute(4, v4, lookup))
	}
	if len(v6) != 0 {
		checks = append(checks, inspectRoute(6, v6, lookup))
	}
	if len(checks) == 0 {
		checks = append(checks, DirectRouteCheck{State: routeUnknown})
	}
	return checks
}

// 扫描前查询各配置网段的代表地址，避免只看系统默认网关。
func routeSamples(cidrs []string) []netip.Addr {
	addresses := make([]netip.Addr, 0, len(cidrs))
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue
		}
		addr := prefix.Masked().Addr()
		if next := addr.Next(); prefix.Contains(next) {
			addr = next
		}
		addresses = append(addresses, addr)
	}
	return addresses
}

func inspectRoute(version int, ips []netip.Addr, lookup func(netip.Addr) routeInterface) DirectRouteCheck {
	check := DirectRouteCheck{Version: version, State: routeDirect}
	if len(ips) == 0 {
		check.State = routeUnknown
	}
	seen := make(map[netip.Addr]bool)
	for _, ip := range ips {
		if seen[ip] {
			continue
		}
		seen[ip] = true
		device := lookup(ip)
		if device.state == routeTunnel {
			return DirectRouteCheck{Version: version, State: routeTunnel, Device: device.name}
		}
		if device.state != routeDirect {
			if check.State != routeUnknown || check.Device == "" {
				check.Device = device.name
			}
			check.State = routeUnknown
		} else if check.State == routeDirect && check.Device == "" {
			check.Device = device.name
		}
	}
	return check
}
