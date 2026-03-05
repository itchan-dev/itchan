package middleware

import (
	"net/http"

	sharedutils "github.com/itchan-dev/itchan/shared/utils"
)

// ReferralConfig holds configuration for the referral tracking middleware.
type ReferralConfig struct {
	SecureCookies bool
	AllowedRefs   sharedutils.AllowedSources
	RecordVisit   func(source string) error
}

// ReferralTracking captures ?ref= param into a cookie on first visit and records the visit.
// AllowedRefs is a pre-built set; empty means allow all sources.
func ReferralTracking(cfg ReferralConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				ref := r.URL.Query().Get("ref")
				if ref != "" {
					if cfg.AllowedRefs.IsAllowed(ref) {
						// Only record if no existing ref cookie (first visit dedup)
						if _, err := r.Cookie("ref"); err != nil {
							http.SetCookie(w, &http.Cookie{
								Name:     "ref",
								Value:    ref,
								Path:     "/",
								MaxAge:   86400 * 30, // 30 days
								HttpOnly: true,
								Secure:   cfg.SecureCookies,
								SameSite: http.SameSiteLaxMode,
							})
							go func(source string) {
								_ = cfg.RecordVisit(source)
							}(ref)
						}
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
