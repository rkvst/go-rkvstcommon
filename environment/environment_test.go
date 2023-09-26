package environment

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRequiredSet(t *testing.T) {
	os.Setenv("ABC", "VAL")
	value, err := GetRequired("ABC")

	assert.Equal(t, "VAL", value)
	assert.Nil(t, err)
}
func TestGetRequiredUnset(t *testing.T) {
	os.Unsetenv("ABC")
	value, err := GetRequired("ABC")

	assert.Equal(t, "", value)
	assert.Equal(t, "required environment variable 'ABC' is not defined", err.Error())
}

// TestGetListOrFatal tests:
//
// 1. a comma separated values (csv) string is correctly separated into a list of values
// 2. a non comma separated values (csv) string is correctly returned as a list with 1 element
func TestGetListOrFatal(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name     string
		args     args
		envKey   string
		envValue string
		expected []string
	}{
		{
			name: "positive csv list",
			args: args{
				key: "SHOPPING",
			},
			envKey:   "SHOPPING",
			envValue: "eggs,flour,milk,sugar,candles,vanillaextract",
			expected: []string{
				"eggs",
				"flour",
				"milk",
				"sugar",
				"candles",
				"vanillaextract",
			},
		},
		{
			name: "positive not csv list",
			args: args{
				key: "BOXERS",
			},
			envKey:   "BOXERS",
			envValue: "mike tyson and rocky",
			expected: []string{
				"mike tyson and rocky",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os.Setenv(test.envKey, test.envValue)

			// ensure we unset the env variable after every test
			t.Cleanup(func() { os.Unsetenv(test.envKey) })

			actual := GetListOrFatal(test.args.key)

			assert.Equal(t, test.expected, actual)

		})
	}
}
