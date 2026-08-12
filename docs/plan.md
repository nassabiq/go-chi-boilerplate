You are working on an EXISTING Go backend project.

Repository:

`github.com/nassabiq/go-chi-boilerplate`

Your job is to continue developing the authentication system from its CURRENT state toward a production-grade authentication module.

IMPORTANT:

- Inspect the repository before making changes.
- Treat the existing codebase as the source of truth.
- Do not blindly create new architecture if an equivalent pattern already exists.
- Reuse existing conventions for:
  - handlers
  - usecases
  - repositories
  - DTOs
  - payloads
  - errors
  - constants
  - config
  - dependency injection
  - migrations
  - NATS
  - Redis
  - PostgreSQL
  - tests
- Do not rewrite working authentication functionality unnecessarily.
- Keep each feature independently testable.
- Do not implement all future phases blindly in one massive change.
- Work PHASE BY PHASE.
- After completing each phase, run tests/build/static checks and report the result before continuing.
- If a prerequisite is missing or the current architecture differs from this prompt, adapt to the repository instead of forcing this exact structure.

---

## CURRENT AUTHENTICATION STATE

The project already has, or is expected to have, the following authentication capabilities:

### Login

Existing:

`POST /auth/login`

Features:

- email normalization
- validation
- login throttling/rate limiting
- generic invalid credentials
- dummy bcrypt comparison for non-existing users
- password verification
- user status policy
- session creation
- access token
- refresh token
- refresh token rotation

Current login status policy:

```text
ACTIVE
→ login allowed

INACTIVE
→ ErrCodeAuthUserInactive

PENDING
→ ErrCodeAuthUserPending

SUSPENDED
→ ErrCodeAuthUserSuspended

BLOCKED
→ ErrCodeAuthUserBlocked
```

Do not break this behavior.

---
## SESSION MANAGEMENT

Existing:

- session creation
- session index per user
- session listing
- session validation
- refresh-token rotation
- single-session logout
- logout-all
- revoked-before mechanism
- IP address metadata
- User-Agent metadata
- stale session cleanup

Existing endpoints conceptually include:

```
POST /auth/logout
POST /auth/logout-all
```

and a user session listing capability.

Do not redesign session management unless a concrete security/correctness issue is discovered.

---
## AUTH ME

Existing:

`GET /auth/me`

This endpoint should return the authenticated user's current identity/profile information.
Do not duplicate authentication logic in this endpoint.

---
## PASSWORD RECOVERY

The project has implemented or is implementing:

```
Forgot Password
Reset Password
Change Password
```

Forgot Password uses:

```
HTTP
↓
PostgreSQL reset token
↓
NATS event
↓
Mail Worker
↓
SMTP
↓
MailDev
```

Password reset tokens are stored securely using hashes rather than raw tokens.

Raw tokens must never be logged.

---

## NATS / EMAIL INFRASTRUCTURE

Existing infrastructure:

- NATS
- NATS JetStream
- APP_EVENTS stream
- domain-first topics
- messaging contracts
- publisher
- mail worker
- SMTP sender
- MailDev

Existing password-reset topic:

```
auth.password-reset.requested.v1
```

Do not replace this architecture.

Future email-based auth events should reuse this infrastructure.

---
## PRIMARY OBJECTIVE

Complete the authentication lifecycle in a logical order.

The desired authentication lifecycle is:

```
REGISTER
   ↓
PENDING
   ↓
EMAIL VERIFICATION
   ↓
ACTIVE
   ↓
LOGIN
   ↓
SESSIONS
   ↓
PASSWORD MANAGEMENT
   ↓
ACCOUNT SECURITY
   ↓
EMAIL / ACCOUNT MANAGEMENT
   ↓
OPTIONAL MFA
   ↓
AUTH HARDENING
```

Implement this in phases.

---

## PHASE 1 — REGISTRATION

Implement:

```
POST /auth/register
```

Request should conceptually contain:

```
{
  "name": "Natsu",
  "email": "user@example.com",
  "password": "..."
}
```

Inspect existing User model/schema before implementing.

Registration flow:

```
normalize email
↓
validate request
↓
validate password policy
↓
check email uniqueness
↓
hash password
↓
create user
status = PENDING
↓
create email verification token
↓
publish verification event
↓
return registration response
```

Requirements:

- new users must NOT become ACTIVE immediately
- new users must start as PENDING
- password must never be stored in plaintext
- email must be normalized consistently
- duplicate email behavior must be handled cleanly
- avoid leaking sensitive internal DB errors
- registration should not send SMTP directly

The HTTP usecase should publish an event.

---
## PHASE 2 — EMAIL VERIFICATION TOKEN

Create a PostgreSQL-backed email verification token model/table.

Conceptually:

```
email_verification_tokens
────────────────────────────
id
user_id
token_hash
expires_at
used_at
requested_ip
user_agent
created_at
```

Adapt fields to the project's existing audit conventions.

Requirements:

- generate cryptographically secure random token
- store only token hash
- raw token appears only in the generated verification URL
- token must expire
- token must be single-use
- token should have audit metadata
- never log raw token
- never return raw token in an API response

Do not use Redis unless there is a concrete architectural reason.

PostgreSQL is the source of truth for verification token history.

---
## PHASE 3 — EMAIL VERIFICATION EVENT

Create a domain-first NATS event:

```
auth.email-verification.requested.v1
```

Contract should conceptually contain:

```
EventID
UserID
Email
Name
VerificationURL
ExpiresAt
OccurredAt
```

Use the project's existing event envelope conventions if available.

Publish through the existing messaging abstraction.

Do not create a second NATS publisher implementation.

---

## PHASE 4 — EMAIL VERIFICATION MAIL

Extend the existing Mail Worker.

The worker should consume:

```
auth.email-verification.requested.v1
```

Reuse:

- NATS JetStream
- durable consumer pattern
- SMTP sender
- MailDev
- ACK/retry semantics
- logging policy

Email must contain:

- greeting
- verification explanation
- verification URL
- expiration
- safe fallback if user did not request it

HTML dynamic values must be escaped.

Never log:

- verification URL
- raw token
- complete event body

---

## PHASE 5 — VERIFY EMAIL

Implement:

```
POST /auth/verify-email
```

Request:

```
{
  "token": "..."
}
```

Flow:

```
hash token
↓
find valid token
↓
check expiry
↓
check used_at
↓
transaction
    ↓
    mark token used
    ↓
    update user status PENDING → ACTIVE
↓
commit
```

Important:

Use transaction + row locking or another concurrency-safe mechanism.

Two concurrent verification requests must not both successfully consume the same token.

Only the correct status transition should activate the account.

Do not blindly set any user to ACTIVE if they are:

- BLOCKED
- SUSPENDED
- INACTIVE

Define and document valid state transitions.

---

## PHASE 6 — RESEND EMAIL VERIFICATION

Implement:

```
POST /auth/resend-verification
```

Request:

```
{
  "email": "user@example.com"
}
```

Requirements:

- normalize email
- generic response to avoid account enumeration
- only resend for users that actually require verification
- avoid generating unlimited tokens
- invalidate or supersede previous verification tokens according to project policy
- rate limit resend attempts
- publish the same domain event
- do not send SMTP directly

Potential policy:

```
PENDING
→ can resend

ACTIVE
→ generic success / no-op

BLOCKED/SUSPENDED/INACTIVE
→ do not reveal status
```

Adapt to existing error/security conventions.

---

## PHASE 7 — PASSWORD MANAGEMENT REVIEW

Now review the existing:

```
Change Password
Reset Password
Forgot Password
```

Do not rewrite working functionality.

Verify:

### Change Password

Authenticated user:

```
current password
new password
```

Requirements:

- verify current password
- validate new password
- prevent reuse if project policy requires it
- hash new password
- invalidate appropriate sessions
- optionally preserve current session according to explicit security policy
- publish password-changed event

### Reset Password

Requirements:

- one-time token
- expiration
- hashed token
- transaction
- invalidate token after use
- update password
- revoke sessions appropriately
- publish password-changed event

### Forgot Password

Verify:

- generic response
- no user enumeration
- token generation
- PostgreSQL persistence
- NATS event
- Mail Worker delivery
- token expiration
- rate limiting

---

## PHASE 8 — PASSWORD CHANGED SECURITY EVENT

Create:

```
auth.password-changed.v1
```

Event should contain safe information such as:

```
EventID
UserID
Email
Name
OccurredAt
Reason
```

Possible Reason values:

```
change_password
reset_password
admin_reset
```

Do NOT include:

- password
- password hash
- reset token
- JWT
- session token

Mail worker consumes the event and sends a security notification.

Example:

```
Your password was changed.

If you did not make this change, please contact support / secure your account.
```

This is a security notification, not a password reset email.

---

## PHASE 9 — EMAIL ADDRESS CHANGE

Implement authenticated email change.

Endpoint conceptually:

```
POST /auth/change-email
```

Request:

```
{
  "new_email": "new@example.com"
}
```

Do NOT immediately change the trusted email address.

Preferred flow:

```
authenticated user
↓
validate password / re-authentication if required
↓
check new email uniqueness
↓
create pending email-change record
↓
generate verification token
↓
publish email-change verification event
↓
user clicks verification
↓
verify token
↓
transaction
    ↓
    update email
    ↓
    invalidate old pending request
↓
commit
```

Consider requiring verification of the NEW email before applying it.

Do not allow an unverified new email to replace the account's trusted email.

---

## PHASE 10 — ACCOUNT LIFECYCLE

Review and implement explicit account lifecycle operations.

Potential endpoints:

```
POST /auth/deactivate
POST /auth/reactivate
DELETE /auth/account
```

Do not implement all blindly.

First inspect the project's user status model and determine which transitions actually make sense.

Define a state machine.

Example:

```
PENDING → ACTIVE
ACTIVE → INACTIVE
ACTIVE → SUSPENDED
ACTIVE → BLOCKED
INACTIVE → ACTIVE
```

But do NOT automatically allow:

```
BLOCKED → ACTIVE
SUSPENDED → ACTIVE
```

unless business rules explicitly permit it.

When account state changes, revoke sessions where appropriate.

---

## PHASE 11 — SESSION SECURITY REVIEW

Review the existing session system after all password/account operations exist.

Expected behavior:

```
Password change
→ revoke appropriate sessions

Password reset
→ revoke ALL sessions

Email change
→ consider session revocation / re-authentication

Account suspension
→ revoke ALL sessions

Account blocking
→ revoke ALL sessions

Account deactivation
→ revoke ALL sessions
```

Do not duplicate session deletion logic.

Reuse:

```
SessionManager.Revoke()
SessionManager.RevokeAll()
```

or improve the abstraction if necessary.

---

## PHASE 12 — SECURITY / ACTIVITY HISTORY

Now add a proper authentication/security activity history if not already present.

Potential event types:

```
login_success
login_failed
logout
logout_all
password_changed
password_reset
email_changed
email_verification
account_deactivated
account_reactivated
session_created
session_revoked
```

Do not store sensitive secrets.

Useful metadata:

```
user_id
event_type
ip_address
user_agent
created_at
metadata
```

Use PostgreSQL for durable audit history.

Do not use Redis as the durable audit store.

Do not log passwords or tokens.

---

## PHASE 13 — MFA / TOTP

ONLY after the previous phases are stable.

Implement optional TOTP MFA.

Do not begin with SMS.

Preferred initial implementation:

```
TOTP
Authenticator App
Recovery Codes
```

Potential flow:

```
enable MFA
↓
generate secret
↓
show provisioning URI / QR
↓
user confirms TOTP
↓
MFA enabled
```

Login becomes:

```
email/password
↓
password correct
↓
MFA enabled?
├── no → create session
└── yes → MFA challenge
          ↓
          verify TOTP
          ↓
          create session
```

Recovery codes must:

- be random
- be hashed at rest
- be one-time use
- never be logged

Do not implement MFA until the basic auth lifecycle is complete.

---

## PHASE 14 — AUTHORIZATION

Only after authentication is stable, move to authorization.

Implement:

```
roles
permissions
role-permission mapping
```

Do not mix authorization logic into login/session code unnecessarily.

Separate:

```
Authentication
→ Who are you?

Authorization
→ What are you allowed to do?
```

Create middleware/policy abstractions appropriate to the project.

---

## PHASE 15 — HARDENING

After features are complete, perform a dedicated authentication hardening pass.

Review:

### Password

- bcrypt/argon2 configuration
- minimum length
- maximum length
- password reuse policy
- timing behavior

### Login

- rate limiting
- brute-force resistance
- account enumeration
- dummy password comparison
- status handling

### Tokens

- cryptographically secure randomness
- token hashing
- expiration
- single-use
- rotation
- revocation

### Sessions

- session IDs
- refresh-token rotation
- reuse detection
- logout
- logout-all
- revoked-before
- stale index cleanup

### Email

- generic forgot-password response
- generic resend-verification response
- secure URLs
- no raw tokens in logs
- no raw tokens in DB
- SMTP failures
- NATS retry

### HTTP

- authentication middleware
- Authorization header handling
- error responses
- CORS
- CSRF considerations if browser cookies are introduced
- security headers

---

## PHASE 16 — AUTH TEST MATRIX

Create comprehensive tests for the completed auth system.

### Registration

Test:

- valid registration
- invalid email
- weak password
- duplicate email
- pending status
- password hashing
- verification event publishing

### Verification

Test:

- valid token
- expired token
- invalid token
- already-used token
- concurrent verification
- status transition
- blocked user
- suspended user

### Login

Test:

- active user
- inactive user
- pending user
- suspended user
- blocked user
- wrong password
- unknown email
- throttled login
- infrastructure failure

### Sessions

Test:

- session creation
- session listing
- logout
- logout-all
- refresh rotation
- token reuse
- revoked-before
- stale sessions

### Password

Test:

- change password
- wrong current password
- reset password
- expired reset token
- reused reset token
- session revocation
- password-changed event

### Email

Test:

- forgot password
- resend verification
- NATS publishing
- consumer
- SMTP success
- SMTP failure
- malformed event
- retry
- ACK behavior

---

## PHASE 17 — AUTH E2E

Create an end-to-end authentication test suite.

Minimum flow:

```
REGISTER
↓
PENDING
↓
verification event
↓
mail worker
↓
MailDev
↓
extract verification link
↓
VERIFY EMAIL
↓
ACTIVE
↓
LOGIN
↓
AUTH/ME
↓
REFRESH
↓
SESSION LIST
↓
LOGOUT
↓
access protected endpoint fails
```

Then:

```
FORGOT PASSWORD
↓
MailDev
↓
RESET PASSWORD
↓
old password fails
↓
new password succeeds
↓
sessions revoked
```

Then:

```
LOGIN
↓
multiple sessions
↓
LOGOUT-ALL
↓
all sessions invalid
```

Use real infrastructure for E2E where practical:

- PostgreSQL
- Redis
- NATS JetStream
- MailDev

Do not fake the entire system with mocks and call that E2E.

---

## PHASE 18 — OBSERVABILITY

After functionality is stable, add useful observability.

Metrics may include:

```
auth.login.success
auth.login.failure
auth.login.throttled

auth.registration.success
auth.email-verification.success

auth.password-reset.requested
auth.password-reset.success
auth.password-reset.failure

auth.session.created
auth.session.revoked

mail.sent
mail.failed
mail.retry
```

Do not put sensitive data into metrics labels.

Avoid labels such as:

```
email
user_id
token
reset_url
```

unless there is a very strong reason.

---

## PHASE 19 — DOCUMENTATION

Update project documentation.

Document:

- auth endpoints
- authentication flow
- token lifecycle
- session lifecycle
- email verification flow
- password reset flow
- NATS topics
- Mail Worker
- local MailDev setup
- environment variables
- account status rules

Create architecture diagrams if the project documentation style supports them.

---

## IMPLEMENTATION RULES

For every phase:

1. Inspect current repository.
2. Identify existing abstractions.
3. Implement the smallest coherent change.
4. Add migration if schema changes are needed.
5. Add unit tests.
6. Add integration tests where appropriate.
7. Run formatting.
8. Run tests.
9. Run static checks.
10. Build affected binaries.
11. Review security implications.
12. Report what changed.

Do not proceed to the next major phase if the current phase is broken.

---

## DATABASE RULES

PostgreSQL is the source of truth for:

- users
- password reset tokens
- email verification tokens
- email change requests
- audit/security history

Use migrations through the project's existing Goose convention.

Do not modify production database schema manually.

Every schema change must have:

- up migration
- down migration if project convention requires it
- indexes where appropriate
- foreign keys where appropriate
- expiration/query indexes for token tables

---

## REDIS RULES

Redis remains appropriate for:

- login throttling
- rate limiting
- ephemeral auth/session data
- session indexes if already implemented

Do not move durable audit/history into Redis.

Do not introduce Redis for email verification tokens merely because Redis is available.

---

## NATS RULES

NATS is for asynchronous events.

Use domain-first topic names:

```
auth.password-reset.requested.v1
auth.password-changed.v1
auth.email-verification.requested.v1
auth.email-changed.v1
```

Do not publish infrastructure-centric topics such as:

```
smtp.send
mail.send
redis.password-reset
```

The auth domain publishes what happened.

The mail worker decides how to deliver it.

---

## ERROR HANDLING RULES

Reuse existing:

```
apperror
errconstant
```

patterns.

Do not introduce random error formats.

HTTP handlers should translate application errors consistently.

Avoid leaking:

- user existence
- database details
- token validity details where enumeration is possible
- SMTP internals
- NATS internals

---

## SECURITY RULES

NEVER log:

- password
- password hash
- raw refresh token
- raw access token
- raw reset token
- raw verification token
- ResetURL
- VerificationURL
- Authorization header

NEVER store raw password-reset or verification tokens in PostgreSQL.

NEVER send passwords through NATS.

NEVER send password hashes through NATS.

NEVER let the mail worker query passwords.

---

## API DESIGN RULES

Use consistent route organization.

Public auth routes:

```
POST /auth/register
POST /auth/login
POST /auth/refresh
POST /auth/forgot-password
POST /auth/reset-password
POST /auth/verify-email
POST /auth/resend-verification
```

Protected auth routes:

```
GET  /auth/me
POST /auth/logout
POST /auth/logout-all
GET  /auth/sessions
POST /auth/change-password
POST /auth/change-email
```

Adapt exact routes to existing project conventions.

Do not create duplicate endpoints.

---

## FINAL TARGET ARCHITECTURE

The finished authentication system should conceptually look like:

```
                    ┌───────────────────┐
                    │     HTTP API      │
                    └─────────┬─────────┘
                              │
                         Auth Usecase
                              │
             ┌────────────────┼─────────────────┐
             │                │                 │
             ▼                ▼                 ▼
        PostgreSQL          Redis             JWT
             │                │                 │
             │                │                 │
             └────────────────┼─────────────────┘
                              │
                         Domain Events
                              │
                              ▼
                       NATS JetStream
                              │
                ┌─────────────┴─────────────┐
                │                           │
                ▼                           ▼
          Mail Worker                 Other Workers
                │
                ▼
          SMTP Sender
                │
                ▼
             MailDev
```

Authentication lifecycle:

```
REGISTER
   ↓
PENDING
   ↓
VERIFY EMAIL
   ↓
ACTIVE
   ↓
LOGIN
   ↓
SESSION
   ↓
┌───────────────┬────────────────┐
│               │                │
PASSWORD        EMAIL            ACCOUNT
MANAGEMENT      MANAGEMENT       MANAGEMENT
│               │                │
↓               ↓                ↓
RESET           CHANGE           DEACTIVATE
CHANGE          VERIFY           SUSPEND
```

---

## PRIORITY ORDER

Do NOT randomly choose the next feature.

Follow this order:

```
PHASE 1
Registration

↓
PHASE 2
Email Verification Token

↓
PHASE 3
Verification NATS Event

↓
PHASE 4
Verification Email

↓
PHASE 5
Verify Email

↓
PHASE 6
Resend Verification

↓
PHASE 7
Review Change/Reset Password

↓
PHASE 8
Password Changed Notification

↓
PHASE 9
Change Email

↓
PHASE 10
Account Lifecycle

↓
PHASE 11
Session Security Review

↓
PHASE 12
Security Activity / Audit

↓
PHASE 13
MFA / TOTP

↓
PHASE 14
Authorization / RBAC

↓
PHASE 15
Security Hardening

↓
PHASE 16
Complete Auth Test Matrix

↓
PHASE 17
Full Auth E2E

↓
PHASE 18
Observability

↓
PHASE 19
Documentation
```

---

## AGENT EXECUTION MODE

Start with PHASE 1 only.

Before modifying anything:

- inspect repository structure
- inspect current user model
- inspect user status enum
- inspect auth routes
- inspect auth handler
- inspect auth usecase
- inspect repository interfaces
- inspect existing migrations
- inspect NATS messaging contracts
- inspect Mail Worker
- inspect error constants
- inspect tests

Then implement Registration.

After PHASE 1:

Run the appropriate commands, for example:

```
gofmt
go test ./...
go vet ./...
go build ./...
```

Also run migrations/tests relevant to the new feature.

Then provide a concise report:

```
PHASE:
Registration

CHANGED:
- files...

DATABASE:
- migration...

API:
- endpoint...

EVENTS:
- published...

TESTS:
- ...

COMMAND RESULTS:
- go test ./...
- go vet ./...
- go build ./...

SECURITY REVIEW:
- ...

REMAINING:
- Email verification
```

STOP after the phase unless the next phase is a trivial prerequisite required for the current phase.

Do NOT implement MFA, RBAC, account lifecycle, or unrelated hardening while working on Registration.

After the user confirms the phase is complete, continue with the next phase.

The goal is not merely to produce code that compiles.

The goal is to build a coherent, secure, testable authentication subsystem with clear boundaries between:

- HTTP
- application/usecase
- persistence
- authentication/session management
- domain events
- NATS
- asynchronous workers
- SMTP
- security auditing
