package err

import "net/http"

var (
	ErrCodeInternal           = response("E_SRV_1", "internal server error", http.StatusInternalServerError)
	ErrCodeServiceUnavailable = response("E_SRV_2", "service unavailable", http.StatusServiceUnavailable)
	ErrCodeTimeout            = response("E_SRV_3", "request timeout", http.StatusGatewayTimeout)
	ErrCodeTooManyRequests    = response("E_SRV_4", "too many requests", http.StatusTooManyRequests)

	ErrCodeStaleResource = response("E_CNF_1", "resource has been modified", http.StatusConflict)

	ErrCodeAuthInvalidCredential        = response("E_AUTH_1", "invalid email or password", http.StatusUnauthorized)
	ErrCodeAuthUnauthorized             = response("E_AUTH_2", "unauthorized", http.StatusUnauthorized)
	ErrCodeAuthForbidden                = response("E_AUTH_3", "forbidden", http.StatusForbidden)
	ErrCodeAuthTokenExpired             = response("E_AUTH_4", "token expired", http.StatusUnauthorized)
	ErrCodeAuthInvalidToken             = response("E_AUTH_5", "invalid token", http.StatusUnauthorized)
	ErrCodeAuthInvalidTokenType         = response("E_AUTH_6", "invalid token type", http.StatusUnauthorized)
	ErrCodeAuthLoginThrottled           = response("E_AUTH_7", "too many login attempts", http.StatusTooManyRequests)
	ErrCodeAuthInvalidRefreshToken      = response("E_AUTH_8", "invalid refresh token", http.StatusUnauthorized)
	ErrCodeAuthUserInactive             = response("E_AUTH_9", "account is inactive", http.StatusForbidden)
	ErrCodeAuthUserPending              = response("E_AUTH_10", "account is pending activation", http.StatusForbidden)
	ErrCodeAuthUserSuspended            = response("E_AUTH_11", "account is suspended", http.StatusForbidden)
	ErrCodeAuthUserBlocked              = response("E_AUTH_12", "account is blocked", http.StatusForbidden)
	ErrCodeAuthInvalidResetToken        = response("E_AUTH_13", "invalid or expired password reset token", http.StatusUnauthorized)
	ErrCodeAuthInvalidCurrentPassword   = response("E_AUTH_14", "current password is incorrect", http.StatusUnauthorized)
	ErrCodeAuthSamePassword             = response("E_AUTH_15", "new password must be different from the current password", http.StatusBadRequest)
	ErrCodeAuthInvalidVerificationToken = response("E_AUTH_16", "invalid or expired email verification token", http.StatusUnauthorized)
	ErrCodeAuthInvalidEmailChangeToken  = response("E_AUTH_17", "invalid or expired email change token", http.StatusUnauthorized)
	ErrCodeAuthEmailUnchanged           = response("E_AUTH_18", "new email must be different from the current email", http.StatusBadRequest)
	ErrCodeAuthInsufficientPermission   = response("E_AUTH_19", "insufficient permissions", http.StatusForbidden)
	ErrCodeAuthRoleNotFound             = response("E_AUTH_20", "role not found", http.StatusNotFound)
	ErrCodeAuthPermissionNotFound       = response("E_AUTH_21", "permission not found", http.StatusNotFound)

	ErrCodeValidation       = response("E_VAL_1", "validation failed", http.StatusBadRequest)
	ErrCodeInvalidRequest   = response("E_VAL_2", "invalid request", http.StatusBadRequest)
	ErrCodeInvalidParameter = response("E_VAL_3", "invalid parameter", http.StatusBadRequest)

	ErrCodeUserNotFound      = response("E_USR_1", "user not found", http.StatusNotFound)
	ErrCodeUserAlreadyExists = response("E_USR_2", "user already exists", http.StatusConflict)
	ErrCodeUserSuspended     = response("E_USR_3", "user suspended", http.StatusForbidden)
	ErrCodeUserBlocked       = response("E_USR_4", "user blocked", http.StatusForbidden)
	ErrCodeUserInactive      = response("E_USR_5", "user inactive", http.StatusForbidden)

	ErrCodeStorageUploadFailed = response("E_STR_1", "failed to upload file to storage", http.StatusInternalServerError)
	ErrCodeStorageFileNotFound = response("E_STR_2", "file not found in storage", http.StatusNotFound)
	ErrCodeStorageInvalidFile  = response("E_STR_3", "invalid file or file size exceeds limit", http.StatusBadRequest)
)

