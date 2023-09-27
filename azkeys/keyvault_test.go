package azkeys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetKeyVersion tests:
//
// 1. with a valid keyvault KID we get the key version back successfully
func TestGetKeyVersion(t *testing.T) {
	type args struct {
		kid string
	}
	tests := []struct {
		name     string
		args     args
		expected string
	}{
		{
			name: "positive",
			args: args{
				kid: "https://example.vault.azure.net/keys/my-key/6eee6743b34e4291807565af6b756bac",
			},
			expected: "6eee6743b34e4291807565af6b756bac",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			actual := GetKeyVersion(test.args.kid)

			assert.Equal(t, test.expected, actual)
		})
	}
}

// TestGetKeyName tests:
//
// 1. with a valid keyvault KID we get the key name back successfully
func TestGetKeyName(t *testing.T) {
	type args struct {
		kid string
	}
	tests := []struct {
		name     string
		args     args
		expected string
	}{
		{
			name: "positive",
			args: args{
				kid: "https://example.vault.azure.net/keys/my-key/6eee6743b34e4291807565af6b756bac",
			},
			expected: "my-key",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			actual := GetKeyName(test.args.kid)

			assert.Equal(t, test.expected, actual)
		})
	}
}
