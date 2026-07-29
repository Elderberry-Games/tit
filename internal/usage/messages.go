package usage

import "fmt"

// Message templates for user-facing source detail lines.
//
// These take provider names, file paths, and environment variable names only.
// Credential material must never be passed to them: every failure message is
// built here so that leaking a token would require adding a new function.

// MsgUnconfigured explains that no credential was found and how to create one.
func MsgUnconfigured(what, howToFix string) string {
	return fmt.Sprintf("no %s found — %s", what, howToFix)
}

// MsgExpired explains that a credential exists but is no longer valid.
func MsgExpired(harness string) string {
	return fmt.Sprintf("credential expired — run `%s` to refresh it", harness)
}

// MsgUnauthorized explains that the credential was rejected.
func MsgUnauthorized(harness string) string {
	return fmt.Sprintf("credential rejected — run `%s` to re-authenticate", harness)
}

// MsgUnreachable explains that the provider could not be reached.
func MsgUnreachable(reason string) string {
	return fmt.Sprintf("could not reach provider (%s) — check your connection and try again", reason)
}

// MsgRateLimited explains that the provider is throttling usage lookups.
func MsgRateLimited() string {
	return "provider is throttling usage lookups — try again in a few minutes"
}

// MsgUnreadable explains that the provider answered with something unusable.
func MsgUnreadable() string {
	return "provider returned an unreadable response — this usually means their API changed"
}

// MsgNoWindows explains that a source works but reports no limit windows.
func MsgNoWindows(what string) string {
	return fmt.Sprintf("%s does not publish rate-limit windows", what)
}
