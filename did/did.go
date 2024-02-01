package did

/**
 * Decentralised Identity (DID) based methods:
 *
 * https://www.w3.org/TR/did-core/
 */

import (
	"crypto"
	"encoding/json"
	"time"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/lestrrat-go/jwx/jwk"
	godid "github.com/nuts-foundation/go-did/did"
)

const (
	didDocumentIDKey      = "id"
	didDocumentTimeout    = time.Second * 30
	DIDFragementDelimiter = "#"
)

// Document represents a DID Document as specified by the DID Core specification (https://www.w3.org/TR/did-core/).
type Document struct {
	ID                 string               `json:"id"`
	Context            []string             `json:"@context,omitempty"`
	Controller         []string             `json:"controller,omitempty"`
	VerificationMethod []VerificationMethod `json:"verificationMethod,omitempty"`
}

// VerificationMethod represents a DID Verification Method as specified by the DID Core specification (https://www.w3.org/TR/did-core/#verification-methods).
type VerificationMethod struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type,omitempty"`
	Controller      string                 `json:"controller,omitempty"`
	PublicKeyBase58 string                 `json:"publicKeyBase58,omitempty"`
	PublicKeyJwk    map[string]interface{} `json:"publicKeyJwk,omitempty"`
}

// did is a Decentralised Identity (DID)
// https://www.w3.org/TR/did-core/
type Did struct {
	did *godid.DID
}

// NewDid returns did given the did URL
//
//	in the form found: https://www.w3.org/TR/did-core/#did-url-syntax
//
// e.g. "did:web:sample.issuer:user:alice#key123"
func NewDid(didURL string) (*Did, error) {

	didID, err := godid.ParseDIDURL(didURL)
	if err != nil {
		// TODO return a suitable error
		return nil, err
	}

	d := Did{
		did: didID,
	}

	return &d, nil
}

// publicKeyFromDocument gets the did's public key based on the id of the did
//
//	from the given document
func (d *Did) publicKeyFromDocument(document *Document) (crypto.PublicKey, error) {

	for _, verificationMethod := range document.VerificationMethod {

		// NOTE: the spec says:
		//  `the verification method map MUST include the id, type, controller`
		//  https://www.w3.org/TR/did-core/#verification-methods`
		//
		//  therefore check for an empty ID
		if verificationMethod.ID == "" {
			logger.Sugar.Infof("publicKeyFromDocument: invalid did, verification methods MUST include id")
			return nil, ErrNoVerificationMethodId
		}

		// check the id of verification method on the docuement is the fragment found in the did
		if verificationMethod.ID != d.did.Fragment {
			continue
		}

		if verificationMethod.PublicKeyJwk == nil {
			continue
		}

		publickKeyJwkJson, err := json.Marshal(verificationMethod.PublicKeyJwk)
		if err != nil {
			logger.Sugar.Infof("publicKeyFromDocument: failed to get public key json: %v", err)
			return nil, err
		}

		publicKeyJwk, err := jwk.ParseKey(publickKeyJwkJson)
		if err != nil {
			logger.Sugar.Infof("publicKeyFromDocument: failed to get public key: %v", err)
			return nil, err
		}

		// we are currently limited by the cose package to using rsa, ecdsa or edwards keys when verifying cose messages
		//  so those are they public key types we will support

		publicKey, err := jwkToPublicKey(publicKeyJwk)
		if err != nil {
			logger.Sugar.Infof("publicKeyFromDocument: failed to get public key from jwk: %v", err)
			return nil, err
		}

		return publicKey, err

	}

	// if we get here, we have evaluated all the verification methods and have not found a match
	logger.Sugar.Infof("publicKeyFromDocument: could not find key by id: %v in did document", d.did.Fragment)
	return nil, &ErrDiDKeyNotFound{keyID: d.did.Fragment}
}
