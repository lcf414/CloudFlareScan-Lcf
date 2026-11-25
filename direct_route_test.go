package main

import (
	"net/netip"
	"slices"
	"testing"
)

func TestRouteSamples(t *testing.T) {
	got := routeSamples([]string{"192.0.2.4/24", "2001:db8::/32", "198.51.100.2/32", "invalid"})
	want := []netip.Addr{
		netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("198.51.100.2"),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("sample addresses = %v, want %v", got, want)
	}
}

func TestInspectRoute(t *testing.T) {
	directIP := netip.MustParseAddr("192.0.2.1")
	unknownIP := netip.MustParseAddr("192.0.2.2")
	tunnelIP := netip.MustParseAddr("192.0.2.3")
	devices := map[netip.Addr]routeInterface{
		directIP:  {state: routeDirect, name: "Ethernet"},
		unknownIP: {state: routeUnknown, name: "Virtual adapter"},
		tunnelIP:  {state: routeTunnel, name: "Wintun"},
	}
	for _, test := range []struct {
		name         string
		ips          []netip.Addr
		state        string
		nameOfDevice string
	}{
		{"direct", []netip.Addr{directIP, directIP}, routeDirect, "Ethernet"},
		{"unknown", []netip.Addr{directIP, unknownIP}, routeUnknown, "Virtual adapter"},
		{"tunnel first", []netip.Addr{unknownIP, directIP, tunnelIP}, routeTunnel, "Wintun"},
		{"no addresses", nil, routeUnknown, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			visited := map[netip.Addr]bool{}
			got := inspectRoute(4, test.ips, func(ip netip.Addr) routeInterface {
				if visited[ip] {
					t.Fatalf("address checked twice: %s", ip)
				}
				visited[ip] = true
				return devices[ip]
			})
			want := DirectRouteCheck{Version: 4, State: test.state, Device: test.nameOfDevice}
			if got != want {
				t.Errorf("route = %+v, want %+v", got, want)
			}
		})
	}
}
