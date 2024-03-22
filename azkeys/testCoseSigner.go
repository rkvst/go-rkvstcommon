package azkeys

// testCoseSigner.go contains an implementation of the IdentifiableCoseSigner and
// IdentifiableCoseSignerFactory interfaces to enable unit testing. The actual signing logic is
// provided by a local COSE signer, wrapped up in a type that conforms to the expectations of the
// service code.

import (
	"context"
	"crypto/ecdsa"
	"io"
	"testing"

	dtcose "github.com/datatrails/go-datatrails-common/cose"
	"github.com/stretchr/testify/require"
	"github.com/veraison/go-cose"
)

type TestCoseSignerFactory struct {
	t          *testing.T
	signingKey ecdsa.PrivateKey
}

func NewTestCoseSignerFactory(t *testing.T, signingKey ecdsa.PrivateKey) IdentifiableCoseSignerFactory {
	return &TestCoseSignerFactory{
		t:          t,
		signingKey: signingKey,
	}
}

func (f *TestCoseSignerFactory) NewIdentifiableCoseSigner(ctx context.Context) (IdentifiableCoseSigner, error) {
	return NewTestCoseSigner(f.t, f.signingKey), nil
}

// TestCoseSigner implements IdentifiableCoseSigner for use with the factory setup in logconfirmer.
type TestCoseSigner struct {
	innerSigner cose.Signer
	publicKey   ecdsa.PublicKey
}

func NewTestCoseSigner(t *testing.T, signingKey ecdsa.PrivateKey) *TestCoseSigner {
	alg, err := dtcose.CoseAlgForEC(signingKey.PublicKey)
	require.NoError(t, err)

	signer, err := cose.NewSigner(alg, &signingKey)
	require.NoError(t, err)

	return &TestCoseSigner{
		innerSigner: signer,
		publicKey:   signingKey.PublicKey,
	}
}

func (s *TestCoseSigner) Algorithm() cose.Algorithm {
	return s.innerSigner.Algorithm()
}

func (s *TestCoseSigner) Sign(rand io.Reader, content []byte) ([]byte, error) {
	return s.innerSigner.Sign(rand, content)
}

func (s *TestCoseSigner) PublicKey() (*ecdsa.PublicKey, error) {
	return &s.publicKey, nil
}

func (s *TestCoseSigner) KeyLocation() string {
	return "test"
}

func (s *TestCoseSigner) KeyIdentifier() string {
	return "foo/bar"
}
