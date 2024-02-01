package did

import (
	"crypto"

	dtcose "github.com/datatrails/go-datatrails-common/cose"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/veraison/go-cose"
)

// Implements a Verify public key provider for did web

type DIDPublicKeyProvider struct {
	cs     *dtcose.CoseSign1Message
	didWeb *Didweb // facilitates testing
}

// NewDIDPublicKeyProvider returns a provider which fetches a did web key To
// customize the key fetch (or for testing purposes), provide a non-nil didWeb.
// Otherwise one is created for you.
func NewDIDPublicKeyProvider(cs *dtcose.CoseSign1Message, didWeb *Didweb) *DIDPublicKeyProvider {
	return &DIDPublicKeyProvider{cs: cs, didWeb: didWeb}
}

func (p *DIDPublicKeyProvider) DidWeb() (*Didweb, error) {

	if p.didWeb != nil {
		return p.didWeb, nil
	}
	// first find the did base
	didBase, err := p.cs.DidFromProtectedHeader()
	if err != nil {
		logger.Sugar.Infof("setDidWeb: failed to get did from header: %v", err)
		return nil, err
	}

	// now find the key id for verification
	kid, err := p.cs.KidFromProtectedHeader()
	if err != nil {
		logger.Sugar.Infof("setDidWeb: failed to get kid from header: %v", err)
		return nil, err
	}

	didURL := didBase + DIDFragementDelimiter + kid
	logger.Sugar.Infof("setDidWeb: didURL: %s", didURL)

	// create the did web
	didweb, err := NewDidWeb(didURL)
	if err != nil {
		logger.Sugar.Infof("setDidWeb: failed to get did from url: %v", err)
		return nil, err
	}

	return didweb, nil
}

func (p *DIDPublicKeyProvider) PublicKey() (crypto.PublicKey, cose.Algorithm, error) {
	didweb, err := p.DidWeb()
	if err != nil {
		return nil, cose.Algorithm(0), err
	}
	protectedHeader := p.cs.Headers.Protected

	// get the algorithm
	algorithm, err := protectedHeader.Algorithm()
	if err != nil {
		// TODO: make an error specific to this and wrap it
		logger.Sugar.Infof("verify: failed to get algorithm: %v", err)
		return nil, cose.Algorithm(0), err
	}

	// find the public key to verify the given message from
	//   the did in the protected header

	publicKey, err := didweb.PublicKey()
	if err != nil {
		logger.Sugar.Infof("verify: failed to get publickey from did: %v", err)
		return nil, cose.Algorithm(0), err
	}

	return publicKey, algorithm, nil
}
