package main

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestClassifyWindowsRoute(t *testing.T) {
	for _, test := range []struct {
		name, description string
		ifType            uint32
		flags             uint8
		state             string
	}{
		{"Ethernet", "Intel Network Adapter", windows.IF_TYPE_ETHERNET_CSMACD, 1, routeDirect},
		{"Wi-Fi", "Wireless Adapter", windows.IF_TYPE_IEEE80211, 1, routeDirect},
		{"Local Network", "Hyper-V Virtual Ethernet", windows.IF_TYPE_ETHERNET_CSMACD, 0, routeUnknown},
		{"Network", "WireGuard Tunnel", windows.IF_TYPE_ETHERNET_CSMACD, 0, routeTunnel},
		{"My adapter", "Virtual", windows.IF_TYPE_TUNNEL, 0, routeTunnel},
		{"VPN 2", "Virtual", windows.IF_TYPE_ETHERNET_CSMACD, 0, routeTunnel},
	} {
		row := windows.MibIfRow2{Type: test.ifType, InterfaceAndOperStatusFlags: test.flags}
		copy(row.Alias[:], windows.StringToUTF16(test.name))
		copy(row.Description[:], windows.StringToUTF16(test.description))
		if device := classifyWindowsRoute(row); device.state != test.state || device.name != test.name {
			t.Errorf("%s: route = %+v, want %s", test.name, device, test.state)
		}
	}
}
