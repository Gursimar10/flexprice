// Package providers is the single registration hub for the generic integration pipeline.
// It lives in its own package (rather than in registry) to avoid a registry<->provider
// import cycle: registry defines the contract, each provider imports registry, and this
// package imports the providers.
package providers

import "github.com/flexprice/flexprice/internal/integration/tabs"

// RegisterAll registers every provider on the generic pipeline. Call once at startup
// (runs in all deployment modes). This list is the source of truth for what is on the new
// pipeline — during migration, add a provider here and remove its legacy triggerXIfEnabled
// + hardcoded dispatch entry in the same change.
func RegisterAll() {
	tabs.Register()
}
