package gatewaysign

// Header constants for gateway signing protocol.
// All headers are case-insensitive in HTTP, but we use the canonical form below
// when generating outbound requests.
const (
	HeaderSign      = "X-Zephyr-Gateway-Sign"
	HeaderTimestamp = "X-Zephyr-Gateway-Ts"
	HeaderNonce     = "X-Zephyr-Gateway-Nonce"
	HeaderRequestID = "X-Zephyr-Request-Id"

	HeaderUserID   = "X-Zephyr-User-Id"
	HeaderUsername = "X-Zephyr-Username"
	HeaderTenantID = "X-Zephyr-Tenant-Id"
	HeaderRoles    = "X-Zephyr-Roles"
)

// AnonymousUser is the user id placeholder when the request did not carry a JWT.
const AnonymousUser = "anonymous"
