package csrf

import (
	"net/http"

	"github.com/gorilla/csrf"
)

// SecureKey is a helper to generate a key if one is not provided.
// It needs to be 32 bytes.
func SecureKey(key []byte) []byte {
	if len(key) >= 32 {
		return key[:32]
	}
	// Fallback/pad to 32 bytes
	pad := make([]byte, 32)
	copy(pad, key)
	return pad
}

// Middleware returns the CSRF protection middleware configured for Qalibre's
// frontend API contract.
func Middleware(secretKey []byte, isDev bool) func(http.Handler) http.Handler {
	opts := []csrf.Option{
		csrf.RequestHeader("X-CSRFToken"),
		csrf.CookieName("csrf_access_token"),
		csrf.Path("/"),
		csrf.SameSite(csrf.SameSiteLaxMode),
		// Since SPA is same-origin, csrf_access_token can be sent with credentials
		csrf.Secure(!isDev),
	}

	protect := csrf.Protect(SecureKey(secretKey), opts...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Exempt API routes that are authenticated via other methods if needed
			// (e.g. Basic Auth for OPDS) or just let gorilla/csrf handle it.
			// OPDS feeds under /opds/ are GET routes, so they are naturally exempt.
			protect(next).ServeHTTP(w, r)
		})
	}
}

// Token returns the CSRF token for the current request.
func Token(r *http.Request) string {
	return csrf.Token(r)
}
