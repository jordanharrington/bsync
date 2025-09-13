package presign

import (
	"context"
	v1 "github.com/jordanharrington/bsync/api/v1"
	"log"
	"time"
)

type PutOptions struct {
	ContentType string
	Metadata    map[string]string
	TTL         time.Duration
	Encryption  *v1.EncryptionSpec
}

// PutOption mutates a PutOptions.
type PutOption func(*PutOptions)

// NewPutOptions applies options over sensible defaults.
func NewPutOptions(opts ...PutOption) PutOptions {
	po := PutOptions{
		Metadata: make(map[string]string),
		TTL:      15 * time.Minute,
	}

	for _, opt := range opts {
		opt(&po)
	}
	return po
}

// WithContentType sets ContentType (empty means provider default).
func WithContentType(ct string) PutOption {
	return func(o *PutOptions) { o.ContentType = ct }
}

// WithMetadata replaces the metadata map (nil = none).
func WithMetadata(md map[string]string) PutOption {
	return func(o *PutOptions) { o.Metadata = md }
}

// WithTTL sets the presign TTL.
func WithTTL(d time.Duration) PutOption {
	return func(o *PutOptions) { o.TTL = d }
}

// WithEncryption sets the v1.EncryptionSpec
func WithEncryption(enc *v1.EncryptionSpec) PutOption {
	return func(o *PutOptions) { o.Encryption = enc }
}

type Presigner interface {
	PresignPut(ctx context.Context, bucket, key string, opts PutOptions) (*v1.PresignedUrl, error)
}

type Registry map[v1.Provider]Presigner

func NewRegistry(ctx context.Context, provider v1.Provider) (Registry, error) {
	aws, err := NewS3Presigner(ctx)
	if err != nil {
		if provider != v1.ProviderAWS {
			log.Printf("could not create AWS presigner: %v", err)
		} else {
			return nil, err
		}
	}

	return map[v1.Provider]Presigner{
		v1.ProviderAWS: aws,
	}, nil
}
