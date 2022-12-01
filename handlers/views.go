package handlers

import (
	"net/http"

	"github.com/go-chi/chi"

	"canvas/views"
)

func FrontPage(mux chi.Router) {
	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_ = views.FrontPage().Render(w)
	})
}
