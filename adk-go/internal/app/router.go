package app

import "net/http"

type Registrar interface {
	RegisterRoutes(*http.ServeMux)
}

func NewRouter(registrars ...Registrar) http.Handler {
	router := http.NewServeMux()
	for _, registrar := range registrars {
		registrar.RegisterRoutes(router)
	}
	return router
}
