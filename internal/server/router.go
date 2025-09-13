package server

import (
	"github.com/gorilla/mux"
	"net/http"
)

type Handler interface {
	HandlePutObject(w http.ResponseWriter, r *http.Request)
}

func NewRouter(h Handler) *mux.Router {
	m := mux.NewRouter().StrictSlash(true)
	v1 := m.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/put", h.HandlePutObject).Methods(http.MethodPost)
	return m
}
