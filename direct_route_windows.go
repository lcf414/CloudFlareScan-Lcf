package main

import (
	"net/netip"
	"strings"

	"golang.org/x/sys/windows"
)

func newRouteLookup() func(netip.Addr) routeInterface {
	interfaces := make(map[uint32]routeInterface)
	return func(ip netip.Addr) routeInterface {
		var sockaddr windows.Sockaddr
		if ip.Is4() {
			sockaddr = &windows.SockaddrInet4{Addr: ip.As4()}
		} else {
			sockaddr = &windows.SockaddrInet6{Addr: ip.As16()}
		}
		var index uint32
		if err := windows.GetBestInterfaceEx(sockaddr, &index); err != nil || index == 0 {
			return routeInterface{state: routeUnknown}
		}
		if device, cached := interfaces[index]; cached {
			return device
		}
		row := windows.MibIfRow2{InterfaceIndex: index}
		if err := windows.GetIfEntry2Ex(windows.MibIfEntryNormalWithoutStatistics, &row); err != nil {
			return routeInterface{state: routeUnknown}
		}
		device := classifyWindowsRoute(row)
		interfaces[index] = device
		return device
	}
}

func classifyWindowsRoute(row windows.MibIfRow2) routeInterface {
	alias := windows.UTF16ToString(row.Alias[:])
	description := windows.UTF16ToString(row.Description[:])
	name := alias
	if name == "" {
		name = description
	}
	text := strings.ToLower(alias + " " + description)
	if row.Type == windows.IF_TYPE_TUNNEL || row.TunnelType != 0 || namedTunnelAdapter(text) {
		return routeInterface{state: routeTunnel, name: name}
	}
	const hardwareInterface = 1 // First bit in MIB_IF_ROW2.InterfaceAndOperStatusFlags.
	if row.InterfaceAndOperStatusFlags&hardwareInterface != 0 &&
		(row.Type == windows.IF_TYPE_ETHERNET_CSMACD || row.Type == windows.IF_TYPE_IEEE80211) {
		return routeInterface{state: routeDirect, name: name}
	}
	return routeInterface{state: routeUnknown, name: name}
}

func namedTunnelAdapter(text string) bool {
	for _, hint := range []string{"wintun", "wireguard", "openvpn", "tap-windows", "tailscale", "mihomo", "clash", "sing-box", "zerotier", "softether", "cloudflare warp"} {
		if strings.Contains(text, hint) {
			return true
		}
	}
	for _, word := range strings.FieldsFunc(text, func(r rune) bool { return r < 'a' || r > 'z' }) {
		if word == "vpn" || word == "tun" || word == "tap" {
			return true
		}
	}
	return false
}
