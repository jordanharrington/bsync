package handlers

import (
	"context"
	v1 "github.com/jordanharrington/bsync/api/v1"
	"github.com/jordanharrington/bsync/internal/presign"
)

type Handler struct {
	Signers presign.Registry
}

func Create(ctx context.Context, provider v1.Provider) (*Handler, error) {
	presignRegistry, err := presign.NewRegistry(ctx, provider)
	if err != nil {
		return nil, err
	}

	return &Handler{
		Signers: presignRegistry,
	}, nil
}
