package messaging

// Domain-first event topics. The auth service publishes the BUSINESS event
// ("a password reset was requested"); it does not instruct a specific mail
// service what to do. Consumers (mail worker, audit worker, security worker)
// subscribe to these topics independently.
const (
	// TopicPasswordResetRequested is published when a user requests a password
	// reset. The mail worker turns it into an email; other workers may treat
	// it as an audit/security signal.
	TopicPasswordResetRequested = "auth.password-reset.requested.v1"

	// TopicPasswordChanged is published after a successful password change or
	// reset, enabling a security-notification email and audit logging.
	TopicPasswordChanged = "auth.password-changed.v1"

	// TopicEmailVerificationRequested is published when a user registers or
	// re-requests verification. The mail worker turns it into a verification
	// email; the raw token lives only inside VerificationURL and is never
	// persisted or logged by the publisher.
	TopicEmailVerificationRequested = "auth.email-verification.requested.v1"

	// TopicEmailChanged is published when an authenticated user requests an
	// email change. It is the NEW-email verification event: the mail worker
	// sends a confirmation link to NewEmail. After the user confirms, the
	// trusted email is swapped. Raw token lives only in VerificationURL.
	TopicEmailChanged = "auth.email-changed.v1"
)
