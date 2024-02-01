package did

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"testing"

	dtcose "github.com/datatrails/go-datatrails-common/cose"
	"github.com/datatrails/go-datatrails-common/did/mocks"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/veraison/go-cose"
)

/*
// VerifyWithDidweb verifies the given message using the given did web to get the public key
//
//	for verification
//
// example code:  https://github.com/veraison/go-cose/blob/main/example_test.go
func (cs *CoseSign1Message) VerifyWithDidweb(external []byte) error {

	return cs.VerifyWithProvider(NewDIDPublicKeyProvider(cs, nil), external)
}*/

// Test_coseSign1Message_VerifyDidweb tests:
//
// 1. we can successfully verify a KAT message using public key from didweb
func Test_coseSign1Message_VerifyDidweb(t *testing.T) {

	logger.New(("NOOP"))
	defer logger.OnExit()

	type fields struct {
		messageb64 string
	}
	type mockWebGetter struct {
		body       string
		statusCode int
		err        error
	}
	type mockDidweb struct {
		didURL string
	}
	tests := []struct {
		name          string
		fields        fields
		mockWebGetter mockWebGetter
		mockDidweb    mockDidweb
		err           error
	}{
		{
			name: "positive",
			fields: fields{
				messageb64: "0oRYVKQBJgRGa2V5LTQyGQGHeDpkaWQ6d2ViOmFwcC5kZXYtamdvdWdoLTAuZGV2LmppdHN1aW4uaW86YXJjaGl2aXN0OnYxOnNjaXR0GQGIZmZvb2JhcqBNeyJmb28iOiJiYXIifVhAWrAfuz5lckMCiWMmk5KEjtWlyGZp2MDOG3XYOPVrOaDXUdw1pM6ZqIit79hmv+pb7inWHCcsbTuzUx6Kb7wI/A==",
			},
			mockDidweb: mockDidweb{
				didURL: "did:web:rkvst.com:archivist:v1:scitt#key-42",
			},
			mockWebGetter: mockWebGetter{
				body:       mockDIDJson,
				statusCode: 200,
				err:        nil,
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

			mDidWeb, err := NewDidWeb(test.mockDidweb.didURL, WithWebGetter(mDidWebGetter))
			require.Nil(t, err)

			// get the cose sign1 message from the KAT base64 message
			base64DecodedStatement, err := base64.StdEncoding.DecodeString(test.fields.messageb64)
			require.Nil(t, err)

			message, err := dtcose.UnmarshalCBOR(base64DecodedStatement)
			require.Nil(t, err)

			cs := &dtcose.CoseSign1Message{
				Sign1Message: message,
			}
			provider := NewDIDPublicKeyProvider(cs, mDidWeb)
			err = cs.VerifyWithProvider(provider, nil)

			assert.Equal(t, test.err, err)
		})
	}
}

// Test_coseSign1Message_VerifyDidweb_negative tests:
//
// 1. no algorithm in protected header, errors
func Test_coseSign1Message_VerifyDidweb_negative(t *testing.T) {

	logger.New(("NOOP"))
	defer logger.OnExit()

	type fields struct {
		payload         []byte
		protectedHeader cose.ProtectedHeader
	}
	type mockWebGetter struct {
		body       string
		statusCode int
		err        error
	}
	type mockDidweb struct {
		didURL string
	}
	tests := []struct {
		name          string
		fields        fields
		mockWebGetter mockWebGetter
		mockDidweb    mockDidweb
		err           error
	}{
		{
			name: "no alg in header",
			fields: fields{
				payload: []byte(`{"foo": "bar"}`),
				protectedHeader: cose.ProtectedHeader{
					// NO ALGORITHM
					391:                   "did:web:app.dev-0.dev.jitsuin.io:archivist:v1:scitt",
					392:                   "foobar",
					cose.HeaderLabelKeyID: []byte("key-42"),
				},
			},
			mockDidweb: mockDidweb{
				didURL: "did:web:rkvst.com:archivist:v1:scitt#key-42",
			},
			mockWebGetter: mockWebGetter{
				body:       mockDIDJson,
				statusCode: 200,
				err:        nil,
			},
			err: errors.New("algorithm not found"),
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

			mDidWeb, err := NewDidWeb(test.mockDidweb.didURL, WithWebGetter(mDidWebGetter))
			require.Nil(t, err)

			// make the message
			message := cose.Sign1Message{
				Headers: cose.Headers{
					Protected: test.fields.protectedHeader,
				},
				Payload: test.fields.payload,
			}

			cs := &dtcose.CoseSign1Message{
				Sign1Message: &message,
			}
			provider := NewDIDPublicKeyProvider(cs, mDidWeb)
			err = cs.VerifyWithProvider(provider, nil)

			assert.Equal(t, test.err, err)
		})
	}
}

// Test_coseSign1Message_DidWeb tests:
//
// 1. the did web is correct based on the did and kid header values in the message
func Test_coseSign1Message_DidWeb(t *testing.T) {

	logger.New(("NOOP"))
	defer logger.OnExit()

	type fields struct {
		message *cose.Sign1Message
	}
	tests := []struct {
		name           string
		fields         fields
		expectedDIDUrl string
		err            error
	}{
		{
			name: "positive",
			fields: fields{
				message: &cose.Sign1Message{
					Headers: cose.Headers{
						Protected: cose.ProtectedHeader{
							dtcose.HeaderLabelDID: "did:web:example.com:foo:bar",
							cose.HeaderLabelKeyID: []byte("key-42"),
						},
					},
				},
			},
			expectedDIDUrl: "did:web:example.com:foo:bar#key-42",
			err:            nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &dtcose.CoseSign1Message{
				Sign1Message: test.fields.message,
			}
			provider := NewDIDPublicKeyProvider(cs, nil)

			didweb, err := provider.DidWeb()

			assert.Equal(t, test.err, err)

			expected, err := NewDidWeb(test.expectedDIDUrl)
			require.Nil(t, err)

			assert.Equal(t, expected, didweb)
		})
	}
}
