package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestReadCloudflareColo(t *testing.T) {
	for _, test := range []struct {
		name, response, want string
	}{
		{"trace body without newline takes priority", "HTTP/1.1 200 OK\r\nContent-Length: 8\r\nCF-Ray: abcdef123456-LAX\r\n\r\ncolo=sin", "SIN"},
		{"chunked trace body", "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n9\r\ncolo=SJC\n\r\n0\r\n\r\n", "SJC"},
		{"header fallback", "HTTP/1.1 403 Forbidden\r\nCF-Ray: 9abcdef123456789-fra\r\nContent-Length: 0\r\n\r\n", "FRA"},
		{"missing data", "HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n", ""},
		{"invalid header", "HTTP/1.1 403 Forbidden\r\nCF-Ray: invalidray-SIN\r\nContent-Length: 0\r\n\r\n", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := readCloudflareColo(strings.NewReader(test.response)); got != test.want {
				t.Errorf("colo = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSpeedResultLogsOneLine(t *testing.T) {
	tester := newSpeedTester([]ScanResult{{IP: "2001:db8::1", Port: 8443, IataCode: "SIN"}}, nil, 0, 0)
	var lines []string
	tester.OnLog = func(line string) { lines = append(lines, line) }
	tester.run(func(string, int, int) float64 { return 1.84 })
	want := []string{"将对 1 个地址与端口组合进行测速", fmt.Sprintf("[1/1] [2001:db8::1]:8443 · 1.84 MB/s · 地区 %s", GetChineseName("SIN"))}
	if !slices.Equal(lines, want) {
		t.Fatalf("log = %q, want %q", lines, want)
	}
}
