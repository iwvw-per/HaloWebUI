package server

import (
	"net/http"
	"strings"
)

const compatibilityFallbackHeader = "X-Halo-Route-Fallback"

// handleCompatibility is a diagnostic fallback only. Every frontend endpoint
// must have an explicit route and a typed handler; unknown legacy paths fail
// loudly instead of persisting arbitrary JSON or fabricating success.
func (a *App) handleCompatibility(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(compatibilityFallbackHeader, "true")
	writeError(w, http.StatusNotFound, "Not found")
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
