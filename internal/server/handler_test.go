package server

import (
	"bytes"
	"context"
	"encoding/json"
	v1 "github.com/jordanharrington/bsync/api/v1"
	"github.com/jordanharrington/bsync/internal/presign"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"

	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"time"
)

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handler")
}

type mockPresigner struct {
	mock.Mock
}

func (m *mockPresigner) PresignPut(ctx context.Context, bucket, key string, opts presign.PutOptions) (*v1.PresignedUrl, error) {
	args := m.Called(ctx, bucket, key, opts)

	return args.Get(0).(*v1.PresignedUrl), args.Error(1)
}

var _ = Describe("Handler", func() {
	var (
		aws *mockPresigner
		hnd *handler
	)

	BeforeEach(func() {
		aws = &mockPresigner{}
		hnd = &handler{
			signers: presign.Registry{
				v1.ProviderAWS: aws,
			},
		}
	})

	type putTestCase struct {
		req                v1.PutObjectRequest
		mockSetup          func()
		expectHTTP         int
		expectTargets      int
		expectErrSubstr    string
		expectPresignCalls int
	}

	DescribeTable("PresignPut",
		func(tc putTestCase) {
			if tc.mockSetup != nil {
				tc.mockSetup()
			}

			bs, _ := json.Marshal(tc.req)
			req := httptest.NewRequest(http.MethodPost, "/v1/presign/put", bytes.NewReader(bs))
			rr := httptest.NewRecorder()
			hnd.handlePutObject(rr, req)

			raw := append([]byte(nil), rr.Body.Bytes()...)
			var resp v1.PutObjectResponse
			if rr.Code == http.StatusOK {
				Expect(rr.Header().Get("Content-Type")).To(ContainSubstring("application/json"))
				_ = json.Unmarshal(raw, &resp)
			}

			Expect(rr.Code).To(Equal(tc.expectHTTP), "body: %s", rr.Body.String())

			Expect(resp.Targets).To(HaveLen(tc.expectTargets))
			for _, t := range resp.Targets {
				Expect(t.URL).NotTo(BeEmpty())
				Expect(t.Headers).NotTo(BeNil())
				Expect(t.TargetRef.Bucket).NotTo(BeEmpty())
				Expect(t.TargetRef.Key).NotTo(BeEmpty())
				Expect(t.TargetRef.Provider).NotTo(BeEmpty())
			}

			if tc.expectErrSubstr != "" {
				Expect(string(raw)).To(ContainSubstring(tc.expectErrSubstr))
			}

			aws.AssertNumberOfCalls(GinkgoT(), "PresignPut", tc.expectPresignCalls)

			aws.AssertExpectations(GinkgoT())
		},

		Entry("success: provider_managed encryption", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{
						Provider: v1.ProviderAWS,
						Bucket:   "b1",
						Key:      "k1",
						Encryption: &v1.EncryptionSpec{
							Type: v1.EncProviderManaged,
						},
					},
				},
			},
			mockSetup: func() {
				optsMatcher := mock.MatchedBy(func(o presign.PutOptions) bool {
					return o.Encryption != nil && o.Encryption.Type == v1.EncProviderManaged && o.Encryption.KeyRef == ""
				})
				aws.
					On("PresignPut", mock.Anything, "b1", "k1", optsMatcher).
					Return(&v1.PresignedUrl{
						TargetRef: v1.TargetRef{Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1"},
						URL:       "https://signed/provider-managed",
						Headers:   map[string]string{"ok": "1"},
					}, nil).
					Once()
			},
			expectHTTP:         http.StatusOK,
			expectTargets:      1,
			expectPresignCalls: 1,
		}),

		Entry("success: customer_managed encryption with key_ref", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{
						Provider: v1.ProviderAWS,
						Bucket:   "b1",
						Key:      "k1",
						Encryption: &v1.EncryptionSpec{
							Type:   v1.EncCustomerManaged,
							KeyRef: "arn:aws:kms:us-east-1:111122223333:key/abcd-ef",
						},
					},
				},
			},
			mockSetup: func() {
				optsMatcher := mock.MatchedBy(func(o presign.PutOptions) bool {
					return o.Encryption != nil &&
						o.Encryption.Type == v1.EncCustomerManaged &&
						o.Encryption.KeyRef == "arn:aws:kms:us-east-1:111122223333:key/abcd-ef"
				})
				aws.
					On("PresignPut", mock.Anything, "b1", "k1", optsMatcher).
					Return(&v1.PresignedUrl{
						TargetRef: v1.TargetRef{Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1"},
						URL:       "https://signed/kms",
						Headers:   map[string]string{"ok": "1"},
					}, nil).
					Once()
			},
			expectHTTP:         http.StatusOK,
			expectTargets:      1,
			expectPresignCalls: 1,
		}),

		Entry("validation: provider_managed must not set key_ref", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{
						Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1",
						Encryption: &v1.EncryptionSpec{
							Type:   v1.EncProviderManaged,
							KeyRef: "should-not-be-here",
						},
					},
				},
			},
			expectHTTP:         http.StatusBadRequest,
			expectErrSubstr:    "key_ref must be empty for provider_managed",
			expectPresignCalls: 0,
			expectTargets:      0,
		}),

		Entry("validation: provider_managed must not set customer-supplied fields", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{
						Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1",
						Encryption: &v1.EncryptionSpec{
							Type:                 v1.EncProviderManaged,
							CustomerKeyB64:       "abc",
							CustomerKeySHA256B64: "def",
						},
					},
				},
			},
			expectHTTP:         http.StatusBadRequest,
			expectErrSubstr:    "customer-supplied fields not allowed for provider_managed",
			expectPresignCalls: 0,
			expectTargets:      0,
		}),

		Entry("validation: customer_managed must provide key_ref", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{
						Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1",
						Encryption: &v1.EncryptionSpec{Type: v1.EncCustomerManaged},
					},
				},
			},
			expectHTTP:         http.StatusBadRequest,
			expectErrSubstr:    "key_ref required for customer_managed",
			expectPresignCalls: 0,
			expectTargets:      0,
		}),

		Entry("validation: customer_managed must not set customer-supplied fields", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{
						Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1",
						Encryption: &v1.EncryptionSpec{
							Type:                 v1.EncCustomerManaged,
							KeyRef:               "arn:aws:kms:us-east-1:111122223333:key/abcd-ef",
							CustomerKeyB64:       "abc",
							CustomerKeySHA256B64: "def",
						},
					},
				},
			},
			expectHTTP:         http.StatusBadRequest,
			expectErrSubstr:    "customer-supplied fields not allowed for customer_managed",
			expectPresignCalls: 0,
			expectTargets:      0,
		}),

		Entry("validation: unsupported encryption type", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{
						Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1",
						Encryption: &v1.EncryptionSpec{Type: "totally_unknown"},
					},
				},
			},
			expectHTTP:         http.StatusBadRequest,
			expectErrSubstr:    "unsupported encryption type",
			expectPresignCalls: 0,
			expectTargets:      0,
		}),
	)
})
