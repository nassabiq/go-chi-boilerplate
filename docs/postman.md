# Postman Collection & Step-by-Step Guide

This guide explains how to import, configure, and execute the complete authentication and security workflow using the provided Postman collection.

---

## 1. Files Provided

- **Collection File**: [`docs/postman_collection.json`](postman_collection.json)
- **Environment File**: [`docs/postman_environment.json`](postman_environment.json)

---

## 2. Importing into Postman

1. Open Postman.
2. Click **Import** (top left).
3. Drag and drop both:
    - `docs/postman_collection.json`
    - `docs/postman_environment.json`
4. Select the imported environment **"Go Chi Boilerplate (Local)"** from the environment dropdown (top right).

---

## 3. Step-by-Step Execution Workflow

The collection is organized in sequential folders. You can execute requests manually one by one or run the entire suite using Postman's **Collection Runner**.

### Step 1: Registration & Verification (`01. Registration & Verification`)

1. **1.1 Register New Account**:
    - Automatically generates a unique email timestamp (e.g. `user_1724838000000@example.com`).
    - Registers user with status `PENDING` (1).
    - Triggers an asynchronous email verification event to NATS.
2. **1.2 Fetch Verification Token from MailDev**:
    - Queries `GET http://localhost:1080/email` (MailDev REST API).
    - Automatically parses the verification link from the email HTML/Text and saves `{{verification_token}}` into collection variables.
3. **1.3 Verify Email Address**:
    - Sends `POST /auth/verify-email` with `{{verification_token}}`.
    - Transitions account to `ACTIVE` (2).
4. **1.4 Resend Verification (Optional)**:
    - Tests the anti-enumeration resend endpoint.

---

### Step 2: Authentication & Profile (`02. Authentication & Token Management`)

1. **2.1 Login**:
    - Authenticates using `{{user_email}}` and `{{user_password}}`.
    - Saves `{{access_token}}` and `{{refresh_token}}` to variables.
2. **2.2 Get User Profile (/auth/me)**:
    - Calls protected route with `Authorization: Bearer {{access_token}}`.
    - Verifies user profile data and `status_id: 2`.
3. **2.3 Refresh Tokens**:
    - Rotates `{{refresh_token}}` and automatically updates both access and refresh tokens.

---

### Step 3: Session Management (`03. Session Management`)

1. **3.1 List Active Sessions**:
    - Retrieves active sessions from Redis.
    - Saves the active `{{session_id}}` to variables.
2. **3.2 Revoke Single Session by ID**:
    - Sends `DELETE /auth/sessions/{{session_id}}`.
3. **3.3 Logout Current Session**:
    - Sends `POST /auth/logout` to revoke current session.
4. **3.4 Logout All Sessions**:
    - Sends `POST /auth/logout-all` to invalidate all sessions across all devices via Redis `revoked_before` marker.

---

### Step 4: Password Management (`04. Password Management`)

1. **4.1 Forgot Password Request**:
    - Sends `POST /auth/forgot-password`.
    - Publishes `auth.password-reset.requested.v1` event to NATS.
2. **4.2 Fetch Reset Token from MailDev**:
    - Automatically retrieves the latest reset email from MailDev and extracts `{{reset_token}}`.
3. **4.3 Reset Password**:
    - Resets password to `{{new_password}}`.
4. **4.4 Login with New Password**:
    - Verifies login with new password succeeds and updates session variables.
5. **4.5 Change Password (Authenticated)**:
    - Changes password while logged in via `POST /auth/change-password`.

---

### Step 5: Email Change (`05. Email Change Flow`)

1. **5.1 Request Email Change**:
    - Sends `POST /auth/change-email` with `{{new_email}}`.
2. **5.2 Fetch Email Change Token from MailDev**:
    - Extracts `{{email_change_token}}` from confirmation email.
3. **5.3 Confirm Email Change**:
    - Confirms email address update via `POST /auth/verify-email-change`.

---

### Step 6: Account Lifecycle (`06. Account Lifecycle`)

1. **6.1 Deactivate Account**:
    - Deactivates current account (`ACTIVE` -> `INACTIVE`) and revokes all sessions.
2. **6.2 Delete Account**:
    - Soft-deletes user account and revokes active sessions.

---

### Step 7: User Management (`07. User Management`)

1. **7.1 List Users**:
    - Paginated query `GET /users?skip=0&limit=10`.
2. **7.2 Get User by ID**:
    - Retrieves user detail by ID `GET /users/{{target_user_id}}`.
3. **7.3 Create User**:
    - Creates user directly via admin API `POST /users`.
4. **7.4 Update User**:
    - Updates user profile fields via `PUT /users/{{target_user_id}}`.
5. **7.5 Delete User (Soft Delete)**:
    - Soft deletes user with optimistic concurrency lock via `DELETE /users/{{target_user_id}}`.
6. **7.6 Restore User**:
    - Restores deleted user via `POST /users/{{target_user_id}}/restore`.
7. **7.7 Update Avatar**:
    - Uploads and attaches user avatar file via `PUT /users/avatar`.

---

### Step 8: Storage & Asset Management (`08. Storage & File Management`)

1. **8.1 Get Presigned Upload URL**:
    - Generates temporary presigned upload URL via `POST /storage/presign-upload`.
2. **8.2 Direct File Upload**:
    - Direct multipart file upload via `POST /storage/upload`.
3. **8.3 Get Presigned Preview URL**:
    - Generates presigned download/view URL via `GET /storage/presign-preview`.
4. **8.4 Delete File from Storage**:
    - Deletes file from storage bucket via `DELETE /storage/file`.

---

### Step 9: Role-Based Access Control (`09. Role-Based Access Control (RBAC)`)

1. **9.1 Create Role**: `POST /roles`
2. **9.2 Get Role by Code**: `GET /roles/admin`
3. **9.3 Create Permission**: `POST /permissions`
4. **9.4 Get Permission by Code**: `GET /permissions/users.read`
5. **9.5 Assign Permission to Role**: `POST /roles/manager/permissions`
6. **9.6 Get Role Permissions**: `GET /roles/admin/permissions`
7. **9.7 Assign Role to User**: `POST /users/{{target_user_id}}/roles`
8. **9.8 Get User Roles**: `GET /users/{{target_user_id}}/roles`
9. **9.9 Get User Permissions**: `GET /users/{{target_user_id}}/permissions`
10. **9.10 Check User Permission**: `GET /users/{{target_user_id}}/permissions/check?permission=users.read`
11. **9.11 Remove Role from User**: `DELETE /users/{{target_user_id}}/roles/manager`

---

### Step 10: Health & Observability (`10. Health & Observability`)

1. **10.1 Liveness Probe**:
    - Kubernetes liveness probe `GET /health/live`.
2. **10.2 Readiness Probe**:
    - Deep readiness probe checking PostgreSQL, Redis, and NATS `GET /health/ready`.
3. **10.3 Legacy Healthz**:
    - Backward-compatible endpoint `GET /healthz`.
4. **10.4 Scrape Prometheus Metrics**:
    - Scrapes OpenTelemetry metrics at `GET /metrics`.

---

## 4. Automated Testing with Collection Runner

1. Click on the collection name **"Go Chi Boilerplate REST API"** in Postman.
2. Click **Run**.
3. Ensure the environment is set to **"Go Chi Boilerplate (Local)"**.
4. Click **Run Go Chi Boilerplate REST API**.
5. All tests will run sequentially, passing tokens and data automatically!
