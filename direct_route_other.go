//go:build !windows

package main

import "net/netip"

func newRouteLookup() func(netip.Addr) routeInterface {
	return func(netip.Addr) routeInterface { return routeInterface{state: routeUnknown} }
}
