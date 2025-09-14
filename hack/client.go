package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	v1 "github.com/jordanharrington/bsync/api/v1"
	"io"
	"net/http"
	"os"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

func main() {
	apiURL := flag.String("api", "", "API Gateway presign endpoint (required)")
	region := flag.String("region", "", "AWS region (required)")
	bucket := flag.String("bucket", "", "S3 bucket (required)")
	key := flag.String("key", "", "S3 key (required)")
	kms := flag.String("kms", "", "KMS key alias/ARN (required)")
	ct := flag.String("content-type", "application/json", "Content-Type for object")
	ttl := flag.Duration("ttl", 2*time.Minute, "Presign TTL")
	payloadPath := flag.String("payload", "", "Path to payload file (required)")

	flag.Parse()

	for name, val := range map[string]string{
		"api":     *apiURL,
		"region":  *region,
		"bucket":  *bucket,
		"key":     *key,
		"kms":     *kms,
		"payload": *payloadPath,
	} {
		if val == "" {
			_, _ = fmt.Fprintf(os.Stderr, "missing required flag: -%s\n", name)
			os.Exit(1)
		}
	}

	ctx := context.Background()

	payload, err := os.ReadFile(*payloadPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to read payload file: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to load AWS config: %v\n", err)
		os.Exit(1)
	}

	reqBody := v1.PutObjectRequest{
		ContentType:   *ct,
		ExpiresMillis: ttl.Milliseconds(),
		ReplicationTargets: []v1.TargetRef{
			{
				Provider: v1.ProviderAWS,
				Bucket:   *bucket,
				Key:      *key,
				Encryption: &v1.EncryptionSpec{
					Type:   v1.EncCustomerManaged,
					KeyRef: *kms,
				},
			},
		},
	}
	bs, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequest(http.MethodPost, *apiURL, bytes.NewReader(bs))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create request: %v\n", err)
		os.Exit(1)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	signer := v4.NewSigner()
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to retrieve creds: %v\n", err)
		os.Exit(1)
	}

	h := sha256.New()
	h.Write(bs)
	payloadHash := hex.EncodeToString(h.Sum(nil))

	err = signer.SignHTTP(ctx, creds, httpReq, payloadHash, "execute-api", *region, time.Now())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to sign request: %v\n", err)
		os.Exit(1)
	}

	client := http.DefaultClient
	resp, err := client.Do(httpReq)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
		os.Exit(1)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "reader closer failed: %v\n", err)
			os.Exit(1)
		}
	}(resp.Body)

	fmt.Printf("Response status: %s\n", resp.Status)
	body, _ := io.ReadAll(resp.Body)

	var out v1.PutObjectResponse
	if err := json.Unmarshal(body, &out); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to parse response: %v\n", err)
		os.Exit(1)
	}

	if len(out.Targets) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "no targets returned")
		os.Exit(1)
	}

	for i, t := range out.Targets {
		if err := putWithPresignedURL(ctx, t, payload, *ct); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "upload %d failed for %s/%s: %v\n", i, t.TargetRef.Bucket, t.TargetRef.Key, err)
			os.Exit(1)
		}
		fmt.Printf("uploaded target %d: %s/%s\n", i, t.TargetRef.Bucket, t.TargetRef.Key)
	}
}

func putWithPresignedURL(ctx context.Context, t v1.PresignedUrl, data []byte, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, t.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", contentType)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to close reader: %v\n", err)
			os.Exit(1)
		}
	}(res.Body)

	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("put failed: %s: %s", res.Status, string(b))
	}

	return nil
}
