package azblob

import (
	"hash"
	"io"

	"github.com/datatrails/go-datatrails-common/logger"
)

type hashingReader struct {
	hasher hash.Hash
	size   int64
	part   io.Reader
}

// Implement reader interface and hash and size file while reading so we can
// retrieve the metadata once the reading is done
func (up *hashingReader) Read(bytes []byte) (int, error) {
	length, err := up.part.Read(bytes)
	if err != nil && err != io.EOF { //nolint https://github.com/golang/go/issues/39155
		logger.Sugar.Errorf("could not read file: %v", err)
		return 0, err
	}
	if length == 0 {
		logger.Sugar.Debugf("finished reading %d bytes", up.size)
		return length, err
	}
	logger.Sugar.Debugf("Read %d bytes (%d)", length, up.size)
	_, herr := up.hasher.Write(bytes[:length])
	if herr != nil {
		logger.Sugar.Errorf("failed to hash")
		return length, herr
	}
	up.size += int64(length)
	if err == io.EOF { //nolint https://github.com/golang/go/issues/39155
		// we've got all of it
		logger.Sugar.Debugf("finished reading %d bytes", up.size)
		return length, err
	}
	return length, nil
}
