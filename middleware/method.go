package middleware

import (
	"net/http"
	"strings"
)

func MethodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Parse form if URL-encoded to get _method field
			contentType := r.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
				_ = r.ParseForm()
			}

			method := r.FormValue("_method")
			if method == "" {
				method = r.URL.Query().Get("_method")
			}

			if method != "" {
				method = strings.ToUpper(method)
				if method == "PATCH" || method == "PUT" || method == "DELETE" {
					r.Method = method
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
