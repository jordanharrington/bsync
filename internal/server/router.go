package server

import (
	"github.com/gorilla/mux"
	"github.com/jordanharrington/bsync/internal/handlers"
	"net/http"
)

func NewRouter(h *handlers.Handler) *mux.Router {
	m := mux.NewRouter().StrictSlash(true)
	v1 := m.PathPrefix("/v1/presign").Subrouter()
	v1.HandleFunc("/put", h.HandlePutObject).Methods(http.MethodPost)
	return m
}
