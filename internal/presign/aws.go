package presign

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	v1 "github.com/jordanharrington/bsync/api/v1"
	"time"
)

type awsPresigner struct {
	client *s3.Client
	signer *s3.PresignClient
}

func NewS3Presigner(ctx context.Context) (Presigner, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	c := s3.NewFromConfig(cfg)
	return &awsPresigner{client: c, signer: s3.NewPresignClient(c)}, nil
}

func (p *awsPresigner) PresignPut(ctx context.Context, bucket, key string, opts PutOptions) (*v1.PresignedUrl, error) {
	in := &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		ContentType: &opts.ContentType,
		ACL:         types.ObjectCannedACLPrivate,
	}

	if len(opts.Metadata) > 0 {
		in.Metadata = map[string]string{}
		for k, v := range opts.Metadata {
			in.Metadata[k] = v
		}
	}

	out, err := p.signer.PresignPutObject(ctx, in, s3.WithPresignExpires(opts.TTL))
	if err != nil {
		return nil, err
	}

	flat := map[string]string{}
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

func (p *awsPresigner) PresignGet(ctx context.Context, bucket, key string, expires time.Duration) (*v1.PresignedUrl, error) {
	if expires <= 0 {
		expires = 15 * time.Minute
	}

	in := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	out, err := p.signer.PresignGetObject(ctx, in, func(po *s3.PresignOptions) {
		po.Expires = expires
	})
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
