package tenantid

import (
	"net/http"
)

const (
	grpcMetadataHeaderKey = "grpc-metadata-archivist-internal-tenant-id"
	headerKey             = "archivist-internal-tenant-id"
	Prefix                = "tenant/"
)

func GetTenantIDFromHeader(h http.Header) string {
	t := h.Get(headerKey)
	if t != "" {
		return t
	}
	t = h.Get(grpcMetadataHeaderKey)
	return t
}

func DeleteTenantIDFromHeader(h http.Header) {
	h.Del(headerKey)
	h.Del(grpcMetadataHeaderKey)
}
