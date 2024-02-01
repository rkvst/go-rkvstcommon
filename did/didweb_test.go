package did

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/datatrails/go-datatrails-common/did/mocks"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	mockDIDJson = `
	{
		"id": "did:web:app.dev-jgough-0.dev.jitsuin.io/archivist/v1/scitt",
		"@context": [
			"https://www.w3.org/ns/did/v1"
		],
		"verificationMethod": [
			{
				"id": "key-42",
				"type": "JsonWebkey",
				"controller": "did:web:app.dev-jgough-0.dev.jitsuin.io/archivist/v1/scitt",
				"publicKeyJwk": {
					"crv":"P-256",
					"kid":"idDrSXgK5zpuctVkmyRQusmP7F1V8C-6BvJL61p93H8",
					"kty":"EC",
					"x":"mQFR6En31DqP5HO6b_iJa-qJQuOd-l2cO5uRgzbw5kA",
					"y":"ynPrCc-5gDizkT4kVIFSvd7w0Y_EawL9-w5umDLiZMs",
					"alg": "ES256"
				}
			}
		]
	}
	`
)

// Test_didweb_URL tests:
//
// 1. we get the correct URL from the did web string
func Test_didweb_URL(t *testing.T) {

	logger.New(("NOOP"))
	defer logger.OnExit()

	type args struct {
		didStr string
	}
	tests := []struct {
		name     string
		args     args
		expected string
		err      error
	}{
		{
			name: "positive",
			args: args{
				didStr: "did:web:sample.issuer:user:alice#key-42",
			},
			expected: "https://sample.issuer/user/alice/did.json",
			err:      nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, err := NewDidWeb(test.args.didStr)
			require.Nil(t, err) // fail the test on a non nil err

			actual, err := d.URL()

			assert.Equal(t, test.err, err)

			urlStr := actual.String()
			assert.Equal(t, test.expected, urlStr)
		})
	}
}

// Test_didweb_DocumentFromWeb tests:
//
// 1. we can get a did document back from a did web string from a mocked did.json
func Test_didweb_DocumentFromWeb(t *testing.T) {

	logger.New(("NOOP"))
	defer logger.OnExit()

	type fields struct {
		didURL string
	}
	type mockWebGetter struct {
		body       string
		statusCode int
		err        error
	}
	tests := []struct {
		name          string
		fields        fields
		mockWebGetter mockWebGetter
		expected      *Document
		err           error
	}{
		{
			name: "positive",
			fields: fields{
				didURL: "did:web:app.dev-jgough-0.dev.jitsuin.io:archivist:v1:scitt#key-42",
			},
			mockWebGetter: mockWebGetter{
				body:       mockDIDJson,
				statusCode: 200,
				err:        nil,
			},
			expected: &Document{
				ID: "did:web:app.dev-jgough-0.dev.jitsuin.io/archivist/v1/scitt",
				Context: []string{
					"https://www.w3.org/ns/did/v1",
				},
				VerificationMethod: []VerificationMethod{
					{
						ID:         "key-42",
						Type:       "JsonWebkey",
						Controller: "did:web:app.dev-jgough-0.dev.jitsuin.io/archivist/v1/scitt",
						PublicKeyJwk: map[string]interface{}{
							"alg": "ES256",
							"crv": "P-256",
							"kid": "idDrSXgK5zpuctVkmyRQusmP7F1V8C-6BvJL61p93H8",
							"kty": "EC",
							"x":   "mQFR6En31DqP5HO6b_iJa-qJQuOd-l2cO5uRgzbw5kA",
							"y":   "ynPrCc-5gDizkT4kVIFSvd7w0Y_EawL9-w5umDLiZMs",
						},
					},
				},
			},
			err: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// mock did web getter
			mDidWebGetter := new(mocks.DidWebGetter)
			mBody := io.NopCloser(bytes.NewBufferString(test.mockWebGetter.body))
			mResponse := http.Response{
				StatusCode: test.mockWebGetter.statusCode,
				Body:       mBody,
			}
			mDidWebGetter.On("Do", mock.Anything).Return(&mResponse, test.mockWebGetter.err).Once()

			d, err := NewDidWeb(test.fields.didURL, WithWebGetter(mDidWebGetter))
			require.Nil(t, err)

			actual, err := d.documentFromWeb()

			assert.Equal(t, test.err, err)
			assert.Equal(t, test.expected, actual)

		})
	}
}
