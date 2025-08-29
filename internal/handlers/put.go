package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/jordanharrington/bsync/api/v1"
	"github.com/jordanharrington/bsync/internal/presign"
	"net/http"
	"time"
)

// HandlePutObject POST /v1/presign/put
func (h *Handler) HandlePutObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var in v1.PutObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	if err := validatePutRequest(in); err != nil {
		http.Error(w, fmt.Sprintf("failed to validate request: %v", err), http.StatusBadRequest)
		return
	}

	ttl := time.Duration(in.ExpiresMillis) * time.Millisecond
	opts := presign.NewPutOptions(
		presign.WithContentType(in.ContentType),
		presign.WithMetadata(in.Metadata),
		presign.WithTTL(ttl),
	)

	urls := make([]v1.PresignedUrl, len(in.ReplicationTargets))
	for _, s := range in.ReplicationTargets {
		presigner, ok := h.Signers[s.Provider]
		if !ok {
			http.Error(w, fmt.Sprintf("provider not configured: %s", s.Provider), http.StatusBadRequest)
			return
		}

		url, err := presigner.PresignPut(ctx, s.Bucket, s.Key, opts)
		if err != nil {
			http.Error(w, fmt.Sprintf("presign failed for %s: %v", s.Provider, err), http.StatusBadGateway)
			return
		}

		urls = append(urls, url)
	}

	_ = json.NewEncoder(w).Encode(v1.PutObjectResponse{
		Targets: urls,
	})
}

var validator = struct {
	minPresignTTL       time.Duration
	maxPresignTTL       time.Duration
	maxMetadataKeys     int
	maxMetadataSize     int
	allowedContentTypes map[string]bool
}{
	minPresignTTL:   1 * time.Minute,
	maxPresignTTL:   10 * time.Minute,
	maxMetadataKeys: 20,
	maxMetadataSize: 2048,
	allowedContentTypes: map[string]bool{
		"application/octet-stream": true,
		"application/json":         true,
		"text/plain":               true,
		"image/png":                true,
		"image/jpeg":               true,
	},
}

func validatePutRequest(in v1.PutObjectRequest) error {
	if in.ContentType == "" || !validator.allowedContentTypes[in.ContentType] {
		return fmt.Errorf("unsupported content type %s", in.ContentType)
	}

	if len(in.Metadata) > validator.maxMetadataKeys {
		return fmt.Errorf("too many metadata entries (max %d)", validator.maxMetadataKeys)
	}

	total := 0
	for k, v := range in.Metadata {
		if k == "" || len(k) > 128 {
			return fmt.Errorf("invalid metadata key: %v. must be 1-128 characters", k)
		}
		if len(v) > 1024 {
			return fmt.Errorf("metadata value too long: %v (max 1024 bytes)", k)
		}
		total += len(k) + len(v)
	}
	if total > validator.maxMetadataSize {
		return fmt.Errorf("metadata with %d entries exceeds max size of %d", total, validator.maxMetadataSize)
	}

	d := time.Duration(in.ExpiresMillis) * time.Millisecond
	if d < validator.minPresignTTL {
		return fmt.Errorf("expires_ms too small (min %d ms)", validator.minPresignTTL.Milliseconds())
	}
	if d > validator.maxPresignTTL {
		return fmt.Errorf("expires_ms too large (max %d ms)", validator.maxPresignTTL.Milliseconds())
	}

	for _, s := range in.ReplicationTargets {
		if s.Bucket == "" || len(s.Bucket) > 63 {
			return fmt.Errorf("invalid bucket name: %s. must be 1-63 characters", s.Bucket)
		}
		if s.Key == "" || len(s.Key) > 1024 {
			return fmt.Errorf("invalid key: %s. must be 1-1024 characters", s.Key)
		}
	}

	return nil
}
