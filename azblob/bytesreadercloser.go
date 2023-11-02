package azblob

import "bytes"

type BytesReaderCloser struct {
	*bytes.Reader
}

func NewBytesReaderCloser(b []byte) *BytesReaderCloser {
	r := &BytesReaderCloser{
		Reader: bytes.NewReader(b),
	}
	return r
}

func (io *BytesReaderCloser) Close() error {
	return nil
}
