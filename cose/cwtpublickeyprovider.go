package cose

import (
	"crypto"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/veraison/go-cose"
)

type CWTPublicKeyProvider struct {
	cs *CoseSign1Message
}

func NewCWTPublicKeyProvider(cs *CoseSign1Message) *CWTPublicKeyProvider {
	return &CWTPublicKeyProvider{cs: cs}
}

func (p *CWTPublicKeyProvider) PublicKey() (crypto.PublicKey, cose.Algorithm, error) {
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

	cwtClaims, err := p.cs.CWTClaimsFromProtectedHeader()
	if err != nil {
		logger.Sugar.Infof("verify: failed to get cwt claims: %v", err)
		return nil, cose.Algorithm(0), err
	}

	if cwtClaims.ConfirmationMethod == nil {
		logger.Sugar.Infof("verify: no verification key in cwt claims: %v", err)
		return nil, cose.Algorithm(0), ErrCWTClaimsNoCNF
	}

	publicKey, err := cwtClaims.ConfirmationMethod.PublicKey()
	if err != nil {
		logger.Sugar.Infof("verify: failed to get publickey from cwt claims: %v", err)
		return nil, cose.Algorithm(0), err
	}

	return publicKey, algorithm, nil
}
