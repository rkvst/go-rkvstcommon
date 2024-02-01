package did

import (
	"context"
	"crypto"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/datatrails/go-datatrails-common/logger"
)

/**
 * did web defines utilities to handle did web documents:
 *
 * https://w3c-ccg.github.io/did-method-web/
 */

const (
	didMethodWeb    = "web"
	didWebScheme    = "https"
	didWebDelimiter = ":"
	didWebResource  = "did.json"
)

// didWebGetter gets a DID document from the web
type didWebGetter interface {
	Do(req *http.Request) (*http.Response, error)
}

// didweb is the did web method implementation:
//
//	https://w3c-ccg.github.io/did-method-web/
type Didweb struct {
	*Did
	webGetter didWebGetter
}

// didwebOption is optional params for creating didwebs
type didwebOption func(*Didweb)

// WithWebGetter sets the web getter, that gets the did document from the web
func WithWebGetter(webGetter didWebGetter) didwebOption {
	return func(dw *Didweb) {
		dw.webGetter = webGetter
	}
}

// NewDidWeb creates a new DID web
func NewDidWeb(didURL string, opts ...didwebOption) (*Didweb, error) {

	did, err := NewDid(didURL)
	if err != nil {
		return nil, err
	}

	d := Didweb{
		Did:       did,
		webGetter: http.DefaultClient, // default to http default client
	}

	// Loop through each option
	for _, opt := range opts {
		opt(&d)
	}

	return &d, nil
}

// URL attempts to take the didVerify's did
//
//	and converts it into a url
//	e.g. "https://sample.issuer/user/alice/did.json"
//
// NOTE: the did has to have method web
func (d *Didweb) URL() (*url.URL, error) {

	if d.Did.did.Method != didMethodWeb {
		logger.Sugar.Infof("URL: not correct method, only support did web")
		return nil, &ErrUnsupportedDIDMethod{method: d.Did.did.Method}
	}

	logger.Sugar.Infof("URL: did: %v", d)

	// we now have to parse the ID of the DID, for example
	// `sample.issuer:user:alice` needs to become
	//  host: sample.issuer
	//  path: /user/alice

	// TODO: as per the spec the path is optional, if there is no path, the resource is
	//       found at the well known endpoint, e.g. `did:web:example.com` == `https://www.example.com/.well-known/did.json`

	// force 2 substrings
	didIDs := strings.SplitN(d.Did.did.ID, didWebDelimiter, 2)
	if len(didIDs) != 2 {
		logger.Sugar.Infof("URL: did id in unexpected format: %v", d.Did.did.ID)
		return nil, &ErrMalformedDIDId{id: d.Did.did.ID}
	}

	host := didIDs[0]
	path := strings.ReplaceAll(didIDs[1], didWebDelimiter, "/")

	// the path needs did.json on the end
	path = path + "/" + didWebResource

	didURL := url.URL{
		Scheme: didWebScheme,
		Host:   host,
		Path:   path,
	}

	logger.Sugar.Infof("URL: did raw url: %v", didURL.String())

	return &didURL, nil
}

// documentFromWeb gets a did document from its corresponding url
//
//	given the did is a did:web
func (d *Didweb) documentFromWeb() (*Document, error) {

	if d.Did.did.Method != didMethodWeb {
		logger.Sugar.Infof("documentFromWeb: did method not supported, currently only web is supported")
		return nil, &ErrUnsupportedDIDMethod{method: d.Did.did.Method}
	}

	// parse the url
	didURL, err := d.URL()
	if err != nil {
		logger.Sugar.Infof("documentFromWeb: cannot find did url: %v", err)
		return nil, err
	}

	logger.Sugar.Infof("documentFromWeb: didURL: %v", didURL)

	// create a context with a timeout so we don't hang forever waiting for the did document
	ctx, cancel := context.WithTimeout(context.Background(), didDocumentTimeout)
	defer cancel()

	urlRaw := didURL.String()
	logger.Sugar.Infof("documentFromWeb: did raw URL: %v", didURL)

	// get the docuement from the url
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, urlRaw, nil)
	if err != nil {
		logger.Sugar.Infof("documentFromWeb: failed to create request for web did: %v", err)
		return nil, err
	}

	response, err := d.webGetter.Do(request)
	if err != nil {
		logger.Sugar.Infof("documentFromWeb: failed to get web did: %v", err)
		return nil, err
	}
	defer response.Body.Close()

	// parse the body of the response into a did document

	var document Document
	err = json.NewDecoder(response.Body).Decode(&document)
	if err != nil {
		logger.Sugar.Infof("documentFromWeb: failed to parse the response: %v, err: %v", response, err)
		return nil, err
	}

	return &document, nil
}

// publicKey gets the public key from the did
func (d *Didweb) PublicKey() (crypto.PublicKey, error) {

	document, err := d.documentFromWeb()
	if err != nil {
		logger.Sugar.Infof("publicKey: failed to get did document from web: %v", err)
		return nil, err
	}

	publicKey, err := d.publicKeyFromDocument(document)
	if err != nil {
		logger.Sugar.Infof("publicKey: failed to get public key from document: %v", err)
		return nil, err
	}

	return publicKey, err
}
