package lnd

import (
	"errors"
	"github.com/lightningnetwork/lnd/tor"
	"net"
	"net/netip"
	"testing"

	"github.com/lightningnetwork/lnd/lncfg"
)

// TestShouldPeerBootstrap tests that we properly skip network bootstrap for
// the developer networks, and also if bootstrapping is explicitly disabled.
func TestShouldPeerBootstrap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		cfg            *Config
		shouldBoostrap bool
	}{
		// Simnet active, no bootstrap.
		{
			cfg: &Config{
				Bitcoin: &lncfg.Chain{
					SimNet: true,
				},
			},
		},

		// Regtest active, no bootstrap.
		{
			cfg: &Config{
				Bitcoin: &lncfg.Chain{
					RegTest: true,
				},
			},
		},

		// Signet active, no bootstrap.
		{
			cfg: &Config{
				Bitcoin: &lncfg.Chain{
					SigNet: true,
				},
			},
		},

		// Mainnet active, but bootstrap disabled, no bootstrap.
		{
			cfg: &Config{
				Bitcoin: &lncfg.Chain{
					MainNet: true,
				},
				NoNetBootstrap: true,
			},
		},

		// Mainnet active, should bootstrap.
		{
			cfg: &Config{
				Bitcoin: &lncfg.Chain{
					MainNet: true,
				},
			},
			shouldBoostrap: true,
		},

		// Testnet active, should bootstrap.
		{
			cfg: &Config{
				Bitcoin: &lncfg.Chain{
					TestNet3: true,
				},
			},
			shouldBoostrap: true,
		},
	}
	for i, testCase := range testCases {
		bootstrapped := shouldPeerBootstrap(testCase.cfg)
		if bootstrapped != testCase.shouldBoostrap {
			t.Fatalf("#%v: expected bootstrap=%v, got bootstrap=%v",
				i, testCase.shouldBoostrap, bootstrapped)
		}
	}
}

func TestParseAddr(t *testing.T) {
	t.Parallel()

	//success cases
	successTestCases := []struct {
		address  string
		netCfg   tor.Net
		expected net.Addr
	}{
		{
			address: "0.0.0.0",
			netCfg:  &tor.ClearNet{},
			expected: net.TCPAddrFromAddrPort(
				netip.MustParseAddrPort("0.0.0.0:9735"))},
		{
			address: "127.0.0.1:1234",
			netCfg:  &tor.ClearNet{},
			expected: net.TCPAddrFromAddrPort(
				netip.MustParseAddrPort("127.0.0.1:1234")),
		},
		{
			address: "255.255.255.255",
			netCfg:  &tor.ClearNet{},
			expected: net.TCPAddrFromAddrPort(
				netip.MustParseAddrPort(
					"255.255.255.255:9735")),
		},
		{
			address: "::1",
			netCfg:  &tor.ClearNet{},
			expected: net.TCPAddrFromAddrPort(
				netip.MustParseAddrPort("[::1]:9735")),
		},
		{
			address: "[2001:db8::1]:1234",
			netCfg:  &tor.ClearNet{},
			expected: net.TCPAddrFromAddrPort(
				netip.MustParseAddrPort("[2001:db8::1]:1234")),
		},
		{
			address: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			netCfg:  &tor.ClearNet{},
			expected: net.TCPAddrFromAddrPort(
				netip.MustParseAddrPort(
					"[ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff]:9735")),
		},
	}

	for _, testCase := range successTestCases {
		t.Run(testCase.address, func(t *testing.T) {
			addr, err := parseAddr(testCase.address, testCase.netCfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if addr == nil {
				t.Fatalf("expected non-nil address")
			}
			if addr.Network() != testCase.expected.Network() {
				t.Fatalf("expected network=%v, got %v", testCase.expected.Network(), addr.Network())
			}
			if addr.String() != testCase.expected.String() {
				t.Fatalf("expected %v, got %v", testCase.expected, addr)
			}
		})
	}

	//failure cases
	failureTestCases := []struct {
		address  string
		netCfg   tor.Net
		expected error
	}{
		{
			address:  "500.0.0.0",
			netCfg:   &tor.ClearNet{},
			expected: errors.New("lookup 500.0.0.0: no such host"),
		},
	}
	for _, testCase := range failureTestCases {
		t.Run(testCase.address, func(t *testing.T) {
			addr, err := parseAddr(testCase.address, testCase.netCfg)
			if err == nil {
				t.Fatalf("expected error: %v, got addr: %v", testCase.expected, addr)
			}
			if err.Error() != testCase.expected.Error() {
				t.Fatalf("expected error: %v, got error: %v", testCase.expected, err)
			}
		})
	}
}
