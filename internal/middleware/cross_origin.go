package middleware

import "net/http"

// CrossOriginProtection applies Go's Fetch Metadata and Origin based CSRF
// protection to browser state-changing requests. Requests without browser
// cross-site headers (for example CLI, bearer-token and internal clients) stay
// compatible with the standard library policy.
func CrossOriginProtection(next http.Handler, trustedOrigins []string) (http.Handler, error) {
	protection := http.NewCrossOriginProtection()
	for _, origin := range trustedOrigins {
		if err := protection.AddTrustedOrigin(origin); err != nil {
			return nil, err
		}
	}
	return protection.Handler(next), nil
}
