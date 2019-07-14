package handlers

import (
	"net/http"

	"github.com/delving/hub3/pkg/namespace"
	"github.com/go-chi/chi"
	"github.com/go-chi/render"
)

func RegisterNamespace(router chi.Router) {
	router.Get("/api/namespaces", listNameSpaces)
}

var svc *namespace.Service

// listNameSpaces list all currently defined NameSpace object
func listNameSpaces(w http.ResponseWriter, r *http.Request) {
	if svc == nil {
		var err error
		svc, err = namespace.NewService(namespace.WithDefaults())
		if err != nil {
			return
		}

	}
	namespaces, err := svc.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render.JSON(w, r, namespaces)
	return
}
