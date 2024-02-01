package did

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedDIDKey = errors.New("unsupported did key")

	// ErrNoVerificationMethodId occurs when a did verification method id is empty
	ErrNoVerificationMethodId = errors.New("did verification method id is not set")
)

// ErrUnsupportedDIDMethod occurs when a did url has an unsupported method, e.g. `did:foobar`
type ErrUnsupportedDIDMethod struct {

	// method the given method
	method string
}

// Error implements the error interface
func (e *ErrUnsupportedDIDMethod) Error() string {
	return fmt.Sprintf("unsupported did method: %v", e.method)
}

// ErrMalformedDIDId occurs when a did id is not in the expected format, expected format is host:{path} e.g. `example.com:path:to:resource`
type ErrMalformedDIDId struct {

	// id the given did id
	id string
}

// Error implements the error interface
func (e *ErrMalformedDIDId) Error() string {
	return fmt.Sprintf("did id not in expected format %v", e.id)
}

// ErrDiDKeyNotFound occurs when a did document does not contain the given key in the verification method list
type ErrDiDKeyNotFound struct {

	// keyID the given key id that was not found
	keyID string
}

// Error implements the error interface
func (e *ErrDiDKeyNotFound) Error() string {
	return fmt.Sprintf("no key with id: %v found in did verification method list", e.keyID)
}
