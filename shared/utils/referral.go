package utils

// AllowedSources holds a set of permitted referral source identifiers for O(1) membership tests.
// An empty AllowedSources means no filtering — all sources are accepted.
type AllowedSources map[string]struct{}

// NewAllowedSources builds an AllowedSources from a slice of permitted identifiers.
func NewAllowedSources(sources []string) AllowedSources {
	set := make(AllowedSources, len(sources))
	for _, s := range sources {
		set[s] = struct{}{}
	}
	return set
}

// IsAllowed reports whether ref is a permitted source.
func (a AllowedSources) IsAllowed(ref string) bool {
	if len(a) == 0 {
		return true
	}
	_, ok := a[ref]
	return ok
}
