package azblob

import "bytes"

// BytesSeekableReader closer provides reader that has a No-Op Close and a
// usuable Seek. Because we need Seek, we can't use ioutil.NopCloser
type BytesSeekableReaderCloser struct {
	*bytes.Reader
}

func NewBytesReaderCloser(b []byte) *BytesSeekableReaderCloser {
	r := &BytesSeekableReaderCloser{
		Reader: bytes.NewReader(b),
	}
	return r
}

func (io *BytesSeekableReaderCloser) Close() error {
	return nil
}
