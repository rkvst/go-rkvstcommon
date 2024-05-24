package azblob

// ReaderOptions - optional args for specifying optional behaviour for
//
//	azblob readers
type ReaderOptions struct {
	accountName string

	container string
}

type ReaderOption func(*ReaderOptions)

// WithAccountName sets the azblob account name for the reader
func WithAccountName(accountName string) ReaderOption {
	return func(a *ReaderOptions) {
		a.accountName = accountName
	}
}

// WithContainer sets the azblob container
func WithContainer(container string) ReaderOption {
	return func(a *ReaderOptions) {
		a.container = container
	}
}

// ParseReaderOptions parses the given options into a ReaderOptions struct
func ParseReaderOptions(options ...ReaderOption) ReaderOptions {
	readerOptions := ReaderOptions{}

	for _, option := range options {
		option(&readerOptions)
	}

	return readerOptions
}
