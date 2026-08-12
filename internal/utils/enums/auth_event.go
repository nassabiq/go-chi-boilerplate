package enums

type AuthEventType string

const (
	AuthEventLoginSuccess     AuthEventType = "login_success"
	AuthEventLoginFailed      AuthEventType = "login_failed"
	AuthEventLogout           AuthEventType = "logout"
	AuthEventLogoutAll        AuthEventType = "logout_all"
	AuthEventPasswordChanged  AuthEventType = "password_changed"
	AuthEventPasswordReset    AuthEventType = "password_reset"
	AuthEventEmailChanged     AuthEventType = "email_changed"
	AuthEventEmailVerified    AuthEventType = "email_verified"
	AuthEventAccountDeactivated AuthEventType = "account_deactivated"
	AuthEventAccountDeleted   AuthEventType = "account_deleted"
	AuthEventSessionCreated   AuthEventType = "session_created"
	AuthEventSessionRevoked   AuthEventType = "session_revoked"
)