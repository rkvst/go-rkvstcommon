package cose

import (
	"crypto"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/veraison/go-cose"
)

type PublicKeyProvider struct {
	cs        *CoseSign1Message
	publicKey crypto.PublicKey
}

func NewPublicKeyProvider(cs *CoseSign1Message, publicKey crypto.PublicKey) *PublicKeyProvider {
	return &PublicKeyProvider{cs: cs, publicKey: publicKey}
}

func (p *PublicKeyProvider) PublicKey() (crypto.PublicKey, cose.Algorithm, error) {
	protectedHeader := p.cs.Headers.Protected

	// get the algorithm
	algorithm, err := protectedHeader.Algorithm()
	if err != nil {
		// TODO: make an error specific to this and wrap it
		logger.Sugar.Infof("verify: failed to get algorithm: %v", err)
		return nil, cose.Algorithm(0), err
	}

	return p.publicKey, algorithm, nil
}
