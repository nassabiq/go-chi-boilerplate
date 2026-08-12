package middleware

import "net/http"

// SecurityHeaders adds standard security-related HTTP response headers to
// every response. These headers harden the application against common
// client-side attacks such as clickjacking, MIME-type sniffing, and
// protocol downgrade attacks.
//
// The headers applied are:
//   - X-Content-Type-Options: nosniff  – prevents MIME-type sniffing
//   - X-Frame-Options: DENY              – clickjacking protection
//   - Referrer-Policy: same-origin        – limits referrer information leakage
//   - X-XSS-Protection: 0                 – disables legacy XSS auditor (use CSP instead)
//   - Permissions-Policy                  – restricts powerful browser APIs
//   - Cache-Control                       – prevents caching of sensitive responses
//   - Strict-Transport-Security           – enforces HTTPS (only meaningful when TLS is terminated)
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "same-origin")
			w.Header().Set("X-XSS-Protection", "0")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=()")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

			// Authenticated endpoints and authentication routes return sensitive
			// credentials and tokens that must never be cached by browsers or proxies.
			if len(r.Header.Get("Authorization")) > 0 || len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/auth" {
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			}

			// HSTS is only useful once the service is reachable over TLS.
			// We set it unconditionally with a modest max-age; reverse proxies
			// that terminate TLS can adjust or remove it.
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")

			next.ServeHTTP(w, r)
		})
	}
}
