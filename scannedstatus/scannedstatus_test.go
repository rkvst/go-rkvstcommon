package scannedstatus

import (
	"net/url"
	"testing"

	"github.com/rkvst/go-rkvstcommon/logger"
	"github.com/stretchr/testify/require"
)

func TestCheckScanStatus(t *testing.T) {
	logger.New("DEBUG")

	table := []struct {
		name          string
		scannedStatus string
		query         url.Values
		success       bool
	}{
		{
			name:          "SCANNED_OK no query success",
			scannedStatus: "SCANNED_OK",
			query:         url.Values{},
			success:       true,
		},
		{
			name:          "SCANNED_OK allow_insecure query success",
			scannedStatus: "SCANNED_OK",
			query:         url.Values{"allow_insecure": []string{"True"}},
			success:       true,
		},
		{
			name:          "SCANNED_OK allow_insecure query success",
			scannedStatus: "SCANNED_OK",
			query:         url.Values{"allow_insecure": []string{"true"}},
			success:       true,
		},
		{
			name:          "SCANNED_OK allow_insecure query numerical success",
			scannedStatus: "SCANNED_OK",
			query:         url.Values{"allow_insecure": []string{"1"}},
			success:       true,
		},
		{
			name:          "SCANNED_OK query numerical success",
			scannedStatus: "SCANNED_OK",
			query:         url.Values{"allow_insecure": []string{"0"}},
			success:       true,
		},
		{
			name:          "SCANNED_OK query false fail",
			scannedStatus: "SCANNED_OK",
			query:         url.Values{"allow_insecure": []string{"false"}},
			success:       true,
		},
		{
			name:          "SCANNED_OK query False fail",
			scannedStatus: "SCANNED_OK",
			query:         url.Values{"allow_insecure": []string{"False"}},
			success:       true,
		},
		{
			name:          "SCANNED_BAD no query fail",
			scannedStatus: "SCANNED_BAD",
			query:         url.Values{},
			success:       false,
		},
		{
			name:          "SCANNED_BAD wrong query fail",
			scannedStatus: "SCANNED_BAD",
			query:         url.Values{"allow_insecure": []string{"maybe"}},
			success:       false,
		},
		{
			name:          "SCANNED_BAD wrong valid query fail",
			scannedStatus: "SCANNED_BAD",
			query:         url.Values{"strict": []string{"true"}},
			success:       false,
		},
		{
			name:          "NOT_SCANNED strict fail",
			scannedStatus: "NOT_SCANNED",
			query:         url.Values{"strict": []string{"true"}},
			success:       false,
		},
		{
			name:          "NOT_SCANNED bad query fail",
			scannedStatus: "NOT_SCANNED",
			query:         url.Values{"strict": []string{"maybe"}},
			success:       false,
		},
		{
			name:          "NOT_SCANNED no query success",
			scannedStatus: "NOT_SCANNED",
			query:         url.Values{},
			success:       true,
		},
		{
			name:          "NOT_SCANNED query strict allow_insecure success",
			scannedStatus: "NOT_SCANNED",
			query:         url.Values{"strict": []string{"true"}, "allow_insecure": []string{"true"}},
			success:       true,
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			err := Check(test.scannedStatus, test.query)
			if test.success {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
			}
		})
	}
}
