package presign

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/samber/lo"

	v1 "github.com/jordanharrington/bsync/api/v1"
)

type s3PresignAPI interface {
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type s3Presigner struct {
	signer s3PresignAPI
}

func NewS3Presigner(ctx context.Context) (Presigner, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg)
	return &s3Presigner{signer: s3.NewPresignClient(client)}, nil
}

func (p *s3Presigner) PresignPut(ctx context.Context, bucket, key string, opts PutOptions) (*v1.PresignedUrl, error) {
	in := &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		ContentType: &opts.ContentType,
		ACL:         types.ObjectCannedACLPrivate,
	}

	if opts.Encryption != nil {
		switch opts.Encryption.Type {
		case v1.EncProviderManaged:
			in.ServerSideEncryption = types.ServerSideEncryptionAes256
		case v1.EncCustomerManaged:
			in.ServerSideEncryption = types.ServerSideEncryptionAwsKms
			in.SSEKMSKeyId = lo.ToPtr(opts.Encryption.KeyRef)
		}
	}

	in.Metadata = make(map[string]string, len(opts.Metadata))
	for k, v := range opts.Metadata {
		in.Metadata[k] = v
	}

	out, err := p.signer.PresignPutObject(ctx, in, s3.WithPresignExpires(opts.TTL))
	if err != nil {
		return nil, err
	}

	flat := make(map[string]string, len(out.SignedHeader))
	for k, vals := range out.SignedHeader {
		if len(vals) > 0 {
			flat[k] = vals[0]
		}
	}

	return &v1.PresignedUrl{
		TargetRef: v1.TargetRef{
			Provider: v1.ProviderAWS,
			Bucket:   bucket,
			Key:      key,
		},
		URL:     out.URL,
		Headers: flat,
	}, nil
}
