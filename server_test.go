package lnd

import (
	"errors"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/lightningnetwork/lnd/lnwallet"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnwire"
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

// TestGenNodeAnnouncementSignFailureNoUpdate tests that if signing the node
// announcement fails, the in-memory s.currentNodeAnn is not updated.
func TestGenNodeAnnouncementSignFailureNoUpdate(t *testing.T) {
	t.Parallel()

	s := &server{
		nodeSigner: &mockNodeSigner{
			signMessageErr: errors.New("signing failed"),
		},
		identityKeyLoc: keychain.KeyLocator{},
		currentNodeAnn: &lnwire.NodeAnnouncement{
			Timestamp: uint32(time.Now().Unix()),
			Addresses: []net.Addr{
				&net.TCPAddr{
					IP:   []byte{192, 0, 2, 1},
					Port: 9735,
				},
			},
			Alias:    lnwire.MustParseNodeAlias("test-node"),
			Features: lnwire.NewRawFeatureVector(),
		},
	}

	// Make a copy of the initial node announcement for comparison.
	initialAnn := *s.currentNodeAnn

	// Call genNodeAnnouncement, expecting an error.
	_, err := s.genNodeAnnouncement(nil, func(nodeAnn *lnwire.NodeAnnouncement) {
		nodeAnn.Alias = lnwire.MustParseNodeAlias("test-node-new-name")
	})
	if err == nil {
		t.Fatal("expected an error from genNodeAnnouncement due to signing failure")
	}

	// Check that s.currentNodeAnn was not modified.
	if !reflect.DeepEqual(&initialAnn, s.currentNodeAnn) {
		t.Fatalf("s.currentNodeAnn was modified despite signing failure.\n"+
			"Initial: %+v\nGot: %+v", initialAnn, *s.currentNodeAnn)
	}
}

// mockNodeSigner is a mock implementation of the netann.NodeSigner interface.
type mockNodeSigner struct {
	signMessageErr error
}

var _ lnwallet.MessageSigner = (*mockNodeSigner)(nil)

// SignMessage mocks the SignMessage method.
func (m *mockNodeSigner) SignMessage(keyLoc keychain.KeyLocator,
	msg []byte, doubleHash bool) (*ecdsa.Signature, error) {

	if m.signMessageErr != nil {
		return nil, m.signMessageErr
	}
	// Return a dummy signature if no error is set.
	return &ecdsa.Signature{}, nil
}

// SignCompactMessage mocks the SignCompactMessage method.
func (m *mockNodeSigner) SignMessageCompact(msg []byte, doubleHash bool) ([]byte,
	error) {
	if m.signMessageErr != nil {
		return nil, m.signMessageErr
	}
	// Return a dummy signature if no error is set.
	return []byte{0x01, 0x02, 0x03}, nil
}
