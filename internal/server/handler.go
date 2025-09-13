package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	v1 "github.com/jordanharrington/bsync/api/v1"
	"github.com/jordanharrington/bsync/internal/presign"
	"net/http"
	"time"
)

type handler struct {
	signers presign.Registry
}

// handlePutObject handles http.MethodPost to /v1/presign/put
func (h *handler) handlePutObject(w http.ResponseWriter, r *http.Request) {
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
	urls := make([]v1.PresignedUrl, 0, len(in.ReplicationTargets))
	for _, s := range in.ReplicationTargets {
		presigner, ok := h.signers[s.Provider]
		if !ok {
			http.Error(w, fmt.Sprintf("provider not configured: %s", s.Provider), http.StatusBadRequest)
			return
		}

		opts := presign.NewPutOptions(
			presign.WithContentType(in.ContentType),
			presign.WithMetadata(in.Metadata),
			presign.WithTTL(ttl),
			presign.WithEncryption(s.Encryption),
		)

		url, err := presigner.PresignPut(ctx, s.Bucket, s.Key, opts)
		if err != nil {
			http.Error(w, fmt.Sprintf("presign failed for %s: %v", s.Provider, err), http.StatusBadGateway)
			return
		}

		urls = append(urls, *url)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v1.PutObjectResponse{
		Targets: urls,
	})
}

var pv = struct {
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
	if in.ContentType == "" || !pv.allowedContentTypes[in.ContentType] {
		return fmt.Errorf("unsupported content type %s", in.ContentType)
	}

	if len(in.Metadata) > pv.maxMetadataKeys {
		return fmt.Errorf("too many metadata entries (max %d)", pv.maxMetadataKeys)
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
	if total > pv.maxMetadataSize {
		return fmt.Errorf("metadata with %d entries exceeds max size of %d", total, pv.maxMetadataSize)
	}

	d := time.Duration(in.ExpiresMillis) * time.Millisecond
	if d < pv.minPresignTTL {
		return fmt.Errorf("expires_ms too small (min %d ms)", pv.minPresignTTL.Milliseconds())
	}
	if d > pv.maxPresignTTL {
		return fmt.Errorf("expires_ms too large (max %d ms)", pv.maxPresignTTL.Milliseconds())
	}

	for _, s := range in.ReplicationTargets {
		if s.Bucket == "" || len(s.Bucket) > 63 {
			return fmt.Errorf("invalid bucket name: %s. must be 1-63 characters", s.Bucket)
		}

		if s.Key == "" || len(s.Key) > 1024 {
			return fmt.Errorf("invalid key: %s. must be 1-1024 characters", s.Key)
		}

		if s.Encryption != nil {
			enc := s.Encryption
			switch enc.Type {
			case v1.EncProviderManaged:
				if enc.KeyRef != "" {
					return errors.New("key_ref must be empty for provider_managed")
				}
				if enc.CustomerKeyB64 != "" || enc.CustomerKeySHA256B64 != "" {
					return errors.New("customer-supplied fields not allowed for provider_managed")
				}
			case v1.EncCustomerManaged:
				if enc.KeyRef == "" {
					return errors.New("key_ref required for customer_managed")
				}
				if enc.CustomerKeyB64 != "" || enc.CustomerKeySHA256B64 != "" {
					return errors.New("customer-supplied fields not allowed for customer_managed")
				}
			default:
				return errors.New("unsupported encryption type")
			}
		}
	}

	return nil
}

func NewRouter(ctx context.Context, provider v1.Provider) (*mux.Router, error) {
	presignRegistry, err := presign.NewRegistry(ctx, provider)
	if err != nil {
		return nil, err
	}

	h := handler{signers: presignRegistry}
	m := mux.NewRouter().StrictSlash(true).PathPrefix("/v1").Subrouter()
	m.HandleFunc("/put", h.handlePutObject).Methods(http.MethodPost)

	return m, nil
}
