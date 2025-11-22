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

type DeleteOptions struct {
	TTL     time.Duration
	Version string
	IfMatch string
}

type Presigner interface {
	PresignPut(ctx context.Context, bucket, key string, opts PutOptions) (*v1.PresignedUrl, error)
	PresignDelete(ctx context.Context, bucket, key string, opts DeleteOptions) (*v1.PresignedUrl, error)
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
