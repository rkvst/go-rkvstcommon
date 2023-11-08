package azblob

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datatrails/go-datatrails-common/logger"
)

func TestHashingReader(t *testing.T) {
	logger.New("NOOP")
	defer logger.OnExit()
	check := assert.New(t)

	data := "Or Lobster Thermidor aux crevettes with a Mornay sauce " +
		"Served in a Provençale manner with shallots and aubergines " +
		"Garnished with truffle pâté, brandy and a fried egg on top and Spam"

	dataReader := strings.NewReader(data)
	hasher := sha256.New()
	reader := &hashingReader{
		hasher: hasher,
		part:   dataReader,
	}

	all := []byte{}
	b := make([]byte, 8)
	for {
		n, err := reader.Read(b)
		all = append(all, b[:n]...)
		if errors.Is(err, io.EOF) {
			break
		}
	}

	check.Equal(
		string(all), data,
	)

	var h [sha256.Size]byte
	hasher.Sum(h[:0])
	hex.EncodeToString(h[:])

	check.Equal(
		hex.EncodeToString(h[:]), "30dff912c17003a5122f09c9a0320a4077614e8b6f107c795c9792b7d963544d",
	)
}
