package server

import (
	"context"
	"github.com/gorilla/mux"
	v1 "github.com/jordanharrington/bsync/api/v1"
	"github.com/jordanharrington/bsync/internal/presign"
	"net/http"
)

func NewRouter(ctx context.Context, provider v1.Provider) (*mux.Router, error) {
	presignRegistry, err := presign.NewRegistry(ctx, provider)
	if err != nil {
		return nil, err
	}

	h := handler{signers: presignRegistry}
	m := mux.NewRouter().StrictSlash(true).PathPrefix("/v1/presign").Subrouter()
	m.HandleFunc("/put", h.handlePutObject).Methods(http.MethodPost)
	m.HandleFunc("/delete", h.handleDeleteObject).Methods(http.MethodPost)

	return m, nil
}
