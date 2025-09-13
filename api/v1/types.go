package v1

type Provider string

const (
	ProviderAWS   Provider = "aws"
	ProviderAzure Provider = "azure"
	ProviderGCP   Provider = "gcp"
)

type EncryptionType string

const (
	EncProviderManaged EncryptionType = "provider_managed"
	EncCustomerManaged EncryptionType = "customer_managed"
)

type EncryptionSpec struct {
	Type                 EncryptionType `json:"type,omitempty"`
	KeyRef               string         `json:"key_ref,omitempty"`
	CustomerKeyB64       string         `json:"customer_key_b64,omitempty"`
	CustomerKeyMD5B64    string         `json:"customer_key_md5_b64,omitempty"`
	CustomerKeySHA256B64 string         `json:"customer_key_sha256_b64,omitempty"`
}

type TargetRef struct {
	Provider   Provider        `json:"provider"`
	Bucket     string          `json:"bucket"`
	Key        string          `json:"key"`
	Encryption *EncryptionSpec `json:"encryption"`
}

type PutObjectRequest struct {
	ReplicationTargets []TargetRef       `json:"replication_targets,omitempty"`
	ContentType        string            `json:"content_type,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	ExpiresMillis      int64             `json:"expires_ms,omitempty"`
}

type PresignedUrl struct {
	TargetRef TargetRef         `json:"target"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type PutObjectResponse struct {
	Targets []PresignedUrl `json:"targets"`
}
