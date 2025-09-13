package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	v1 "github.com/jordanharrington/bsync/api/v1"
	"github.com/jordanharrington/bsync/internal/presign"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"time"
)

type mockPresigner struct {
	mock.Mock
}

func (m *mockPresigner) PresignPut(ctx context.Context, bucket, key string, opts presign.PutOptions) (*v1.PresignedUrl, error) {
	args := m.Called(ctx, bucket, key, opts)

	return args.Get(0).(*v1.PresignedUrl), args.Error(1)
}

type putTestCase struct {
	name            string
	req             v1.PutObjectRequest
	mockSetup       func()
	expectHTTP      int
	expectTargets   int
	expectErrSubstr string
}

var _ = Describe("Handler", func() {
	var (
		aws *mockPresigner
		hnd *handler
	)

	BeforeEach(func() {
		aws = &mockPresigner{}
		hnd = &handler{
			Signers: presign.Registry{
				v1.ProviderAWS: aws,
			},
		}
	})

	DescribeTable("PresignPut",
		func(tc putTestCase) {
			if tc.mockSetup != nil {
				tc.mockSetup()
			}

			bs, _ := json.Marshal(tc.req)
			req := httptest.NewRequest(http.MethodPost, "/v1/presign/put", bytes.NewReader(bs))
			rr := httptest.NewRecorder()
			hnd.HandlePutObject(rr, req)

			var resp v1.PutObjectResponse
			_ = json.NewDecoder(rr.Body).Decode(&resp)

			Expect(rr.Code).To(Equal(tc.expectHTTP), "body: %s", rr.Body.String())
			Expect(resp.Targets).To(HaveLen(tc.expectTargets))
			for _, t := range resp.Targets {
				Expect(t.URL).NotTo(BeEmpty())
				Expect(t.Headers).NotTo(BeEmpty())
				Expect(t.TargetRef).NotTo(BeEmpty())
			}

			if tc.expectErrSubstr != "" {
				Expect(rr.Body.String()).To(ContainSubstring(tc.expectErrSubstr))
			}

			aws.AssertExpectations(GinkgoT())
		},

		Entry("success with two targets", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				Metadata:      map[string]string{"a": "b"},
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1"},
					{Provider: v1.ProviderAWS, Bucket: "b1", Key: "k2"},
				},
			},
			mockSetup: func() {
				aws.
					On("PresignPut", mock.Anything, "b1", "k1", mock.AnythingOfType("presign.PutOptions")).
					Return(&v1.PresignedUrl{TargetRef: v1.TargetRef{Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1"}, URL: "https://signed/1"}, nil).
					Once()
				aws.
					On("PresignPut", mock.Anything, "b1", "k2", mock.AnythingOfType("presign.PutOptions")).
					Return(&v1.PresignedUrl{TargetRef: v1.TargetRef{Provider: v1.ProviderAWS, Bucket: "b1", Key: "k2"}, URL: "https://signed/2"}, nil).
					Once()
			},
			expectHTTP:    http.StatusOK,
			expectTargets: 2,
		}),

		Entry("validation: bad content type", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/x-bad",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1"},
				},
			},
			expectHTTP:      http.StatusBadRequest,
			expectErrSubstr: "unsupported content type",
		}),

		Entry("unknown provider", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{Provider: v1.ProviderAzure, Bucket: "b1", Key: "k1"},
				},
			},
			expectHTTP:      http.StatusBadRequest,
			expectErrSubstr: "provider not configured",
		}),

		Entry("presign error", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1"},
				},
			},
			mockSetup: func() {
				aws.
					On("PresignPut", mock.Anything, "b1", "k1", mock.AnythingOfType("presign.PutOptions")).
					Return((*v1.PresignedUrl)(nil), errors.New("boom")).
					Once()
			},
			expectHTTP:      http.StatusBadGateway,
			expectErrSubstr: "presign failed",
		}),

		Entry("validation: ttl too small", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (30 * time.Second).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{Provider: v1.ProviderAWS, Bucket: "b1", Key: "k1"},
				},
			},
			expectHTTP:      http.StatusBadRequest,
			expectErrSubstr: "expires_ms too small",
		}),

		Entry("validation: invalid bucket/key", putTestCase{
			req: v1.PutObjectRequest{
				ContentType:   "application/json",
				ExpiresMillis: (2 * time.Minute).Milliseconds(),
				ReplicationTargets: []v1.TargetRef{
					{Provider: v1.ProviderAWS, Bucket: "", Key: "k1"},
				},
			},
			expectHTTP:      http.StatusBadRequest,
			expectErrSubstr: "invalid bucket",
		}),
	)
})
