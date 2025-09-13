package presign

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	v1 "github.com/jordanharrington/bsync/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"net/http"
	"testing"
	"time"
)

func TestAWS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS")
}

type mockS3PresignAPI struct {
	mock.Mock
}

func (m *mockS3PresignAPI) PresignPutObject(
	ctx context.Context,
	in *s3.PutObjectInput,
	optFns ...func(*s3.PresignOptions),
) (*v4.PresignedHTTPRequest, error) {
	args := m.Called(ctx, in, optFns)

	return args.Get(0).(*v4.PresignedHTTPRequest), args.Error(1)
}

var _ = Describe("S3", func() {
	var (
		ctx context.Context
		m   *mockS3PresignAPI
		ps  Presigner
	)

	BeforeEach(func() {
		ctx = context.Background()
		m = &mockS3PresignAPI{}
		ps = &s3Presigner{signer: m}
	})

	type putTestCase struct {
		name        string
		bucket      string
		key         string
		ct          string
		meta        map[string]string
		ttl         time.Duration
		enc         *v1.EncryptionSpec
		mockURL     string
		mockHeaders http.Header
		mockErr     error
		wantErr     bool
		wantURL     string
		wantHdrPick map[string]string
		wantSSE     types.ServerSideEncryption
		wantKMS     *string
		wantCalls   int
	}

	DescribeTable("PresignPut",
		func(tc putTestCase) {
			var retResp *v4.PresignedHTTPRequest
			if tc.mockErr == nil {
				h := make(http.Header, len(tc.mockHeaders))
				for k, vals := range tc.mockHeaders {
					for _, v := range vals {
						h.Add(k, v)
					}
				}
				retResp = &v4.PresignedHTTPRequest{
					URL:          tc.mockURL,
					SignedHeader: h,
				}
			}
			m.
				On("PresignPutObject", mock.Anything, mock.AnythingOfType("*s3.PutObjectInput"), mock.Anything).
				Run(func(args mock.Arguments) {
					in := args.Get(1).(*s3.PutObjectInput)

					// bucket/key
					Expect(in.Bucket).NotTo(BeNil())
					Expect(in.Key).NotTo(BeNil())
					Expect(*in.Bucket).To(Equal(tc.bucket))
					Expect(*in.Key).To(Equal(tc.key))

					// content-type (pointer always set by impl)
					Expect(in.ContentType).NotTo(BeNil())
					Expect(*in.ContentType).To(Equal(tc.ct))

					// metadata (impl always creates map; may be empty)
					if tc.meta == nil {
						Expect(in.Metadata).To(Equal(map[string]string{}))
					} else {
						Expect(in.Metadata).To(Equal(tc.meta))
					}

					// encryption selection
					Expect(in.ServerSideEncryption).To(Equal(tc.wantSSE))
					if tc.wantKMS == nil {
						Expect(in.SSEKMSKeyId).To(BeNil())
					} else {
						Expect(in.SSEKMSKeyId).NotTo(BeNil())
						Expect(*in.SSEKMSKeyId).To(Equal(*tc.wantKMS))
					}

					// TTL propagation check
					optFns, _ := args.Get(2).([]func(*s3.PresignOptions))
					var po s3.PresignOptions
					for _, fn := range optFns {
						fn(&po)
					}
					Expect(po.Expires).To(Equal(tc.ttl))
				}).
				Return(retResp, tc.mockErr).
				Once()

			opts := NewPutOptions(
				WithContentType(tc.ct),
				WithMetadata(tc.meta),
				WithTTL(tc.ttl),
				WithEncryption(tc.enc),
			)

			u, err := ps.PresignPut(ctx, tc.bucket, tc.key, opts)

			if tc.wantErr {
				Expect(err).To(HaveOccurred())
				Expect(u).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(u).NotTo(BeNil())
				Expect(u.URL).To(Equal(tc.wantURL))
				Expect(u.TargetRef.Provider).To(Equal(v1.ProviderAWS))
				Expect(u.TargetRef.Bucket).To(Equal(tc.bucket))
				Expect(u.TargetRef.Key).To(Equal(tc.key))

				for k, v := range tc.wantHdrPick {
					Expect(u.Headers).To(HaveKeyWithValue(k, v))
				}
			}

			m.AssertNumberOfCalls(GinkgoT(), "PresignPutObject", tc.wantCalls)
			m.AssertExpectations(GinkgoT())
		},

		Entry("success: content-type + metadata + ttl (no encryption)",
			putTestCase{
				name:    "ok",
				bucket:  "b1",
				key:     "k1",
				ct:      "application/octet-stream",
				meta:    map[string]string{"a": "b"},
				ttl:     2 * time.Minute,
				enc:     nil,
				mockURL: "https://signed/put?ok=1",
				mockHeaders: http.Header{
					"Content-Type": {"application/octet-stream"},
					"X-Amz-Meta-A": {"b"},
				},
				wantErr:     false,
				wantURL:     "https://signed/put?ok=1",
				wantHdrPick: map[string]string{"Content-Type": "application/octet-stream", "X-Amz-Meta-A": "b"},
				wantSSE:     "", // none
				wantKMS:     nil,
				wantCalls:   1,
			},
		),

		Entry("success: provider_managed -> SSE-S3 (AES256)",
			putTestCase{
				bucket:  "b2",
				key:     "k2",
				ct:      "text/plain",
				meta:    map[string]string{},
				ttl:     90 * time.Second,
				enc:     &v1.EncryptionSpec{Type: v1.EncProviderManaged},
				mockURL: "https://signed/put?pm=1",
				mockHeaders: http.Header{
					"Content-Type": {"text/plain"},
				},
				wantErr:     false,
				wantURL:     "https://signed/put?pm=1",
				wantHdrPick: map[string]string{"Content-Type": "text/plain"},
				wantSSE:     types.ServerSideEncryptionAes256,
				wantKMS:     nil,
				wantCalls:   1,
			},
		),

		Entry("success: customer_managed -> SSE-KMS with key id",
			func() putTestCase {
				k := "arn:aws:kms:us-east-1:111122223333:key/abcd-ef"
				return putTestCase{
					bucket:  "b3",
					key:     "k3",
					ct:      "application/json",
					meta:    map[string]string{"x": "y"},
					ttl:     30 * time.Second,
					enc:     &v1.EncryptionSpec{Type: v1.EncCustomerManaged, KeyRef: k},
					mockURL: "https://signed/put?kms=1",
					mockHeaders: http.Header{
						"Content-Type": {"application/json"},
					},
					wantErr:     false,
					wantURL:     "https://signed/put?kms=1",
					wantHdrPick: map[string]string{"Content-Type": "application/json"},
					wantSSE:     types.ServerSideEncryptionAwsKms,
					wantKMS:     aws.String(k),
					wantCalls:   1,
				}
			}(),
		),

		Entry("success: flattens multi-value headers (first wins)",
			putTestCase{
				bucket:  "b4",
				key:     "k4",
				ct:      "text/plain",
				meta:    map[string]string{},
				ttl:     75 * time.Second,
				enc:     nil,
				mockURL: "https://signed/put?multi=1",
				mockHeaders: http.Header{
					"X-Custom":     {"first", "second"},
					"Content-Type": {"text/plain"},
				},
				wantErr:     false,
				wantURL:     "https://signed/put?multi=1",
				wantHdrPick: map[string]string{"X-Custom": "first", "Content-Type": "text/plain"},
				wantSSE:     "",
				wantKMS:     nil,
				wantCalls:   1,
			},
		),

		Entry("error: AWS SDK presign failure bubbles up",
			putTestCase{
				bucket:    "b5",
				key:       "k5",
				ct:        "application/json",
				meta:      map[string]string{"z": "1"},
				ttl:       45 * time.Second,
				enc:       nil,
				mockErr:   http.ErrHandlerTimeout,
				wantErr:   true,
				wantSSE:   "",
				wantKMS:   nil,
				wantCalls: 1,
			},
		),
	)
})
