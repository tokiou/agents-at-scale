package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Registrar interface {
	Register(chi.Router)
}

func NewRouter(registrars ...Registrar) http.Handler {
	router := chi.NewRouter()
	for _, registrar := range registrars {
		registrar.Register(router)
	}
	return router
}
