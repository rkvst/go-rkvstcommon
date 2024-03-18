package cose

import (
	"crypto"
	"crypto/ecdsa"
	"errors"
	"io"
	"reflect"

	dtcbor "github.com/datatrails/go-datatrails-common/cbor"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/fxamacker/cbor/v2"
	"github.com/ldclabs/cose/go/cwt"
	"github.com/veraison/go-cose"
)

/**
 * Cose functions based on CBOR Object Signing and Encryption (COSE)
 *
 * https://datatracker.ietf.org/doc/html/rfc8152
 */

const (
	HeaderLabelCWTClaims              int64 = 13
	HeaderLabelReceiptVersion         int64 = 390
	HeaderLabelDID                    int64 = 391
	HeaderLabelFeed                   int64 = 392
	HeaderLabelRegistrationPolicyInfo int64 = 393
)

// CoseSign1Message extends the cose.sign1message
type CoseSign1Message struct {
	*cose.Sign1Message
	decMode cbor.DecMode
	encMode cbor.EncMode
}

func newDefaultSignOptions() SignOptions {
	opts := SignOptions{
		// Fill in defaults
		encOpts: &cbor.EncOptions{},
		decOpts: &cbor.DecOptions{},
	}
	*opts.encOpts = dtcbor.NewDeterministicEncOpts()
	*opts.decOpts = dtcbor.NewDeterministicDecOptsConvertSigned()
	return opts
}

// NewCoseSign1Message creates a new cose sign1 message
func NewCoseSign1Message(message *cose.Sign1Message, withOpts ...SignOption) (*CoseSign1Message, error) {

	opts := newDefaultSignOptions()

	for _, o := range withOpts {
		o(&opts)
	}

	var err error

	csm := CoseSign1Message{
		Sign1Message: message,
	}

	csm.encMode, err = opts.encOpts.EncMode()
	if err != nil {
		return nil, err
	}

	csm.decMode, err = opts.decOpts.DecMode()
	if err != nil {
		return nil, err
	}

	return &csm, nil
}

// NewCoseSign1Message creates a new cose sign1 message from a cbor encoded message
func NewCoseSign1MessageFromCBOR(message []byte, withOpts ...SignOption) (*CoseSign1Message, error) {

	opts := newDefaultSignOptions()

	for _, o := range withOpts {
		o(&opts)
	}

	coseMessage, err := UnmarshalCBOR(message)
	if err != nil {
		logger.Sugar.Infof("NewCoseSign1MessageFromCBOR: failed to unmarshal cbor: %v", err)
		return nil, err
	}

	sign1Message := &CoseSign1Message{
		Sign1Message: coseMessage,
	}

	sign1Message.encMode, err = opts.encOpts.EncMode()
	if err != nil {
		return nil, err
	}

	sign1Message.decMode, err = opts.decOpts.DecMode()
	if err != nil {
		return nil, err
	}

	return sign1Message, nil
}

// MarshalCBOR marshals a cose_Sign1 message to cbor
func MarshalCBOR(message *cose.Sign1Message) ([]byte, error) {

	marshaledMessage, err := message.MarshalCBOR()
	if err != nil {
		return nil, err
	}

	return marshaledMessage, err
}

// UnmarshalCBOR unmarshals a cbor encoded cose_Sign1 message
func UnmarshalCBOR(message []byte) (*cose.Sign1Message, error) {

	var unmarshaledMessage cose.Sign1Message
	err := unmarshaledMessage.UnmarshalCBOR(message)
	if err != nil {
		return nil, err
	}

	return &unmarshaledMessage, err
}

// valueFromProtectedHeader gets a value from the cose_Sign1 protected Header given the label
func (cs *CoseSign1Message) valueFromProtectedHeader(label int64) (any, error) {

	header := cs.Headers.Protected

	value, ok := header[label]
	if !ok {
		logger.Sugar.Infof("valueFromProtectedHeader: failed to get value for label: %v", label)
		return nil, &ErrNoProtectedHeaderValue{Label: label}
	}

	return value, nil
}

// ContentTypeFromProtectedheader gets the content type from the given protected header
func (cs *CoseSign1Message) ContentTypeFromProtectedheader() (string, error) {

	contentType, err := cs.valueFromProtectedHeader(cose.HeaderLabelContentType)
	if err != nil {
		logger.Sugar.Infof("ContentTypeFromProtectedheader: failed to get content type from protected header: %v", err)
		return "", err
	}

	contentTypeStr, ok := contentType.(string)
	if !ok {
		logger.Sugar.Infof("didFromProtectedHeader: did from protected header is not string: %v", err)
		return "", &ErrUnexpectedProtectedHeaderType{label: cose.HeaderLabelContentType, expectedType: "string", actualType: reflect.TypeOf(contentType).String()}
	}

	return contentTypeStr, nil
}

// DidFromProtectedHeader gets the DID (Decentralised IDentity)
//
//	to use to acquire the public key for verifying
func (cs *CoseSign1Message) DidFromProtectedHeader() (string, error) {

	did, err := cs.valueFromProtectedHeader(HeaderLabelDID)
	if err != nil {
		logger.Sugar.Infof("DidFromProtectedHeader: failed to get did from protected header: %v", err)
		return "", err
	}

	didStr, ok := did.(string)
	if !ok {
		logger.Sugar.Infof("DidFromProtectedHeader: did from protected header is not string")
		return "", &ErrUnexpectedProtectedHeaderType{label: HeaderLabelDID, expectedType: "string", actualType: reflect.TypeOf(did).String()}
	}

	return didStr, nil
}

// CWTClaimsFromProtectedHeader gets the CWT Claims from the protected header
func (cs *CoseSign1Message) CWTClaimsFromProtectedHeader() (*CWTClaims, error) {

	cwtClaimsRaw, err := cs.valueFromProtectedHeader(HeaderLabelCWTClaims)
	if err != nil {
		logger.Sugar.Infof("CWTClaimsFromProtectedHeader: failed to get cwt claims from protected header: %v", err)
		return nil, err
	}

	cwtClaimsMap, ok := cwtClaimsRaw.(map[interface{}]interface{})
	if !ok {
		logger.Sugar.Infof("CWTClaimsFromProtectedHeader: cwt claims from protected header is not map: %v", err)
		return nil, &ErrUnexpectedProtectedHeaderType{label: HeaderLabelCWTClaims, expectedType: "map[interface{}]interface{}", actualType: reflect.TypeOf(cwtClaimsMap).String()}
	}

	issuer, ok := cwtClaimsMap[int64(cwt.KeyIss)]
	if !ok {
		logger.Sugar.Infof("CWT Claims: %v", cwtClaimsMap)
		logger.Sugar.Infof("CWTClaimsFromProtectedHeader: failed to get issuer from cwt claims: %v", err)
		return nil, ErrCWTClaimsNoIssuer
	}

	issuerStr, ok := issuer.(string)
	if !ok {
		logger.Sugar.Infof("CWT Claims: %v", cwtClaimsMap)
		logger.Sugar.Infof("CWTClaimsFromProtectedHeader: issuer is not string: %v", err)
		return nil, ErrCWTClaimsIssuerNotString
	}

	subject, ok := cwtClaimsMap[int64(cwt.KeySub)]
	if !ok {
		logger.Sugar.Infof("CWT Claims: %v", cwtClaimsMap)
		logger.Sugar.Infof("CWTClaimsFromProtectedHeader: failed to get subject from cwt claims: %v", err)
		return nil, ErrCWTClaimsNoSubject
	}

	subjectStr, ok := subject.(string)
	if !ok {
		logger.Sugar.Infof("CWT Claims: %v", cwtClaimsMap)
		logger.Sugar.Infof("CWTClaimsFromProtectedHeader: subject is not string: %v", err)
		return nil, ErrCWTClaimsSubjectNotString
	}

	cwtClaims := CWTClaims{
		Issuer:  issuerStr,
		Subject: subjectStr,
	}

	// find verification key
	verificationKey, err := CNFCoseKey(cwtClaimsMap)

	if err != nil {

		// cnf is an optional field, so if we don't have one, log but don't error out
		if errors.Is(err, ErrCWTClaimsNoCNF) {
			logger.Sugar.Infof("CWTClaimsFromProtectedHeader: failed to get cnf field: %v", err)
		} else {
			// in this case we have a cnf field, there is some other error
			logger.Sugar.Infof("CWTClaimsFromProtectedHeader: failed to get verification filed: %v", err)
			return nil, err
		}
	}

	if err == nil {
		cwtClaims.ConfirmationMethod = verificationKey
	}

	return &cwtClaims, nil
}

// FeedFromProtectedHeader gets the feed id from the protected header
func (cs *CoseSign1Message) FeedFromProtectedHeader() (string, error) {

	feed, err := cs.valueFromProtectedHeader(HeaderLabelFeed)
	if err != nil {
		logger.Sugar.Infof("feedFromProtectedHeader: failed to get feed from protected header: %v", err)
		return "", err
	}

	feedStr, ok := feed.(string)
	if !ok {
		logger.Sugar.Infof("feedFromProtectedHeader: feed from protected header is not string: %v", err)
		return "", &ErrUnexpectedProtectedHeaderType{label: HeaderLabelFeed, expectedType: "string", actualType: reflect.TypeOf(feed).String()}
	}

	return feedStr, nil
}

// KidFromProtectedHeader gets the  kid from the protected header
func (cs *CoseSign1Message) KidFromProtectedHeader() (string, error) {

	kid, err := cs.valueFromProtectedHeader(cose.HeaderLabelKeyID)
	if err != nil {
		logger.Sugar.Infof("kidFromProtectedHeader: failed to get kid from protected header: %v", err)
		return "", err
	}

	kidBytes, ok := kid.([]byte)
	if !ok {
		logger.Sugar.Infof("kidFromProtectedHeader: kid from protected header is not []byte: %v", err)
		return "", &ErrUnexpectedProtectedHeaderType{label: cose.HeaderLabelKeyID, expectedType: "[]byte", actualType: reflect.TypeOf(kid).String()}
	}

	return string(kidBytes), nil
}

type publicKeyProvider interface {
	PublicKey() (crypto.PublicKey, cose.Algorithm, error)
}

func (cs *CoseSign1Message) VerifyWithProvider(
	pubKeyProvider publicKeyProvider, external []byte) error {

	publicKey, algorithm, err := pubKeyProvider.PublicKey()
	if err != nil {
		return err
	}

	verifier, err := cose.NewVerifier(algorithm, publicKey)
	if err != nil {
		logger.Sugar.Infof("verify: publicKey: %v, algorithm: %v", publicKey, algorithm)
		logger.Sugar.Infof("verify: failed to make verifier from public key: %v", err)
		return err
	}

	// verify the message
	err = cs.Verify(external, verifier)
	if err != nil {
		logger.Sugar.Infof("verify: publicKey: %v, algorithm: %v", publicKey, algorithm)
		logger.Sugar.Infof("verify: failed to verify message: %v", err)
		return err
	}

	return nil
}

// VerifyWithCWTPublicKey verifies the given message using the public key
//
// found in the CWT Claims of the protected header
//
// https://ietf-wg-scitt.github.io/draft-ietf-scitt-architecture/draft-ietf-scitt-architecture.html
//
//		CWT_Claims = {
//		1 => tstr; iss, the issuer making statements,
//		2 => tstr; sub, the subject of the statements, (feed id)
//		/cnf/ 8 = > {
//		  /COSE_Key/ 1 :{
//			/kty/ 1 : /EC2/ 2,
//			/crv/ -1 : /P-256/ 1,
//			/x/ -2 : h'd7cc072de2205bdc1537a543d53c60a6acb62eccd890c7fa27c9
//					   e354089bbe13',
//			/y/ -3 : h'f95e1d4b851a2cc80fff87d8e23f22afb725d535e515d020731e
//					   79a3b4e47120'
//		   }
//		 }
//	 }
//		}
//
// NOTE: that iss needs to be set, as the user needs to trace the given public key back to an issuer.
func (cs *CoseSign1Message) VerifyWithCWTPublicKey(external []byte) error {

	return cs.VerifyWithProvider(NewCWTPublicKeyProvider(cs), external)
}

// VerifyWithPublicKey verifies the given message using the given public key
//
//	for verification
//
// example code:  https://github.com/veraison/go-cose/blob/main/example_test.go
func (cs *CoseSign1Message) VerifyWithPublicKey(publicKey crypto.PublicKey, external []byte) error {
	return cs.VerifyWithProvider(NewPublicKeyProvider(cs, publicKey), external)
}

// SignES256 signs a cose sign1 message using the given ecdsa private key using the algorithm ES256
func (cs *CoseSign1Message) SignES256(rand io.Reader, external []byte, privateKey *ecdsa.PrivateKey) error {
	signer, err := cose.NewSigner(cose.AlgorithmES256, privateKey)
	if err != nil {
		return err
	}

	if cs.Headers.Protected == nil {
		cs.Headers.Protected = make(cose.ProtectedHeader)
	}

	// Note: It *must* be ES256 to work with this types Verify etc. we could
	// detect the programming error where the caller has set the wrong alg but
	// that seems overly fussy.
	cs.Headers.Protected[cose.HeaderLabelAlgorithm] = cose.AlgorithmES256

	return cs.Sign(rand, external, signer)
}
