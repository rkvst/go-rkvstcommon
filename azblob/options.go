package azblob

import "time"

type GetMetadata int

const (
	NoMetadata GetMetadata = iota
	OnlyMetadata
	BothMetadataAndBlob
)

type ETagCondition int

const (
	EtagNotUsed ETagCondition = iota
	ETagMatch
	ETagNoneMatch
	TagsWhere
)

type IfSinceCondition int

const (
	IfConditionNotUsed IfSinceCondition = iota
	IfConditionModifiedSince
	IfConditionUnmodifiedSince
)

func (g GetMetadata) String() string {
	return [...]string{"No metadata handling", "Only metadata", "metadata and blob"}[g]
}

// StorerOptions - optional args for specifying optional behaviour
type StorerOptions struct {
	leaseID        string
	metadata       map[string]string
	tags           map[string]string
	getMetadata    GetMetadata
	getTags        bool
	sizeLimit      int64
	etag           string
	etagCondition  ETagCondition // ETagMatch || ETagNoneMatch
	sinceCondition IfSinceCondition
	since          *time.Time
	// Options for List()
	listPrefix string
	listDelim  string
	listMarker ListMarker
	// extra data model items to include in the respponse
	listIncludeTags     bool
	listIncludeMetadata bool
	// There are more, but these are all we need for now
}
type ListMarker *string

type Option func(*StorerOptions)

func WithListPrefix(prefix string) Option {
	return func(a *StorerOptions) {
		a.listPrefix = prefix
	}
}

// TODO: this is an sdk v1.2.1 feature
func WithListDelim(delim string) Option {
	return func(a *StorerOptions) {
		a.listDelim = delim
	}
}

func WithListMarker(marker ListMarker) Option {
	return func(a *StorerOptions) {
		a.listMarker = marker
	}
}

func WithListTags() Option {
	return func(a *StorerOptions) {
		a.listIncludeTags = true
	}
}

func WithListMetadata() Option {
	return func(a *StorerOptions) {
		a.listIncludeMetadata = true
	}
}

func WithModifiedSince(since *time.Time) Option {
	return func(a *StorerOptions) {
		if since == nil {
			a.sinceCondition = IfConditionNotUsed
			a.since = nil
			return
		}
		a.sinceCondition = IfConditionModifiedSince
		a.since = since
	}
}

func WithUnmodifiedSince(since *time.Time) Option {
	return func(a *StorerOptions) {
		if since == nil {
			a.sinceCondition = IfConditionNotUsed
			a.since = nil
			return
		}
		a.sinceCondition = IfConditionUnmodifiedSince
		a.since = since
	}
}

// WithEtagMatch succeed if the blob etag matches the provied value
// Typically used to make optimistic concurrency updates safe.
func WithEtagMatch(etag string) Option {
	return func(a *StorerOptions) {
		a.etag = etag
		// Only one condition at a time is possible. If multiple are requested, the last one wins
		a.etagCondition = ETagMatch
	}
}

// WithEtagNoneMatch succeed if the blob etag does *not* match the supplied value
func WithEtagNoneMatch(etag string) Option {
	return func(a *StorerOptions) {
		a.etag = etag
		// Only one condition at a time is possible. If multiple are requested, the last one wins
		a.etagCondition = ETagNoneMatch
	}
}

// WithWhereTags succeed if the where clause matches the blob tags)
func WithWhereTags(whereTags string) Option {
	return func(a *StorerOptions) {
		a.etag = whereTags
		// Only one condition at a time is possible. If multiple are requested, the last one wins
		a.etagCondition = TagsWhere
	}
}

// Specifying an option that is no used is silently ignored. i.e. Specifying
// WithMetadata() in a call to Reader() will not raise an error.

// WithLeaseID - specifies LeaseId - Reader() and Write()
func WithLeaseID(leaseID string) Option {
	return func(a *StorerOptions) {
		a.leaseID = leaseID
	}
}

// WithMetadata specifies metadata to add - Write() only
func WithMetadata(metadata map[string]string) Option {
	return func(a *StorerOptions) {
		a.metadata = metadata
	}
}

// WithGetMetadata specifies to get metadata - Reader() only.
func WithGetMetadata(value GetMetadata) Option {
	return func(a *StorerOptions) {
		a.getMetadata = value
	}
}

// WithTags specifies tags to add - Reader() and Write(). For Write) the tags are written
// with the blob. For Reader() the tags are used to apply ownership permissions.
func WithTags(tags map[string]string) Option {
	return func(a *StorerOptions) {
		a.tags = tags
	}
}

func WithGetTags() Option {
	return func(a *StorerOptions) {
		a.getTags = true
	}
}

// WithSizeLimit specifies the size limit of the blob.
// -1 for unlimited. 0+ for limited.
func WithSizeLimit(sizeLimit int64) Option {
	return func(a *StorerOptions) {
		a.sizeLimit = sizeLimit
	}
}
