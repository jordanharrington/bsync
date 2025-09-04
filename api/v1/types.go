package v1

type Provider string

const (
	ProviderAWS   Provider = "aws"
	ProviderAzure Provider = "azure"
	ProviderGCP   Provider = "gcp"
)

type TargetRef struct {
	Provider Provider `json:"provider"`
	Bucket   string   `json:"bucket"`
	Key      string   `json:"key"`
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
