-- +goose Up
-- Seed Roles
INSERT INTO roles (code, name, description)
VALUES
    ('admin', 'Administrator', 'System administrator with full access'),
    ('user', 'User', 'Standard application user')
ON CONFLICT (code) DO NOTHING;

-- Seed Permissions
INSERT INTO permissions (code, name, description)
VALUES
    ('users.read', 'Read Users', 'View user profiles and list users'),
    ('users.write', 'Write Users', 'Create, update, and delete users'),
    ('roles.manage', 'Manage Roles', 'Manage RBAC roles and permissions'),
    ('storage.upload', 'Upload Files', 'Upload assets to object storage')
ON CONFLICT (code) DO NOTHING;

-- Assign all permissions to admin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;

-- Assign standard permissions to user
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN ('users.read', 'storage.upload')
WHERE r.code = 'user'
ON CONFLICT DO NOTHING;

-- Seed default Superadmin user (Password: password)
INSERT INTO users (name, email, password, status_id, version, created_at, updated_at)
VALUES (
    'Super Administrator',
    'admin@example.com',
    '$2a$12$aPolNt3W8p19JhOz6EqIb.iP52Zd.RUFUVG.4OPY7flI1Isg.UyZ2',
    1, -- ACTIVE
    1,
    NOW(),
    NOW()
)
ON CONFLICT (email) DO NOTHING;

-- Assign admin role to Superadmin
INSERT INTO user_roles (user_id, role_id, assigned_at)
SELECT u.id, r.id, NOW()
FROM users u
JOIN roles r ON r.code = 'admin'
WHERE u.email = 'admin@example.com'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE email = 'admin@example.com');
DELETE FROM activity_logs WHERE user_id IN (SELECT id FROM users WHERE email = 'admin@example.com');
DELETE FROM password_reset_tokens WHERE user_id IN (SELECT id FROM users WHERE email = 'admin@example.com');
DELETE FROM email_verification_tokens WHERE user_id IN (SELECT id FROM users WHERE email = 'admin@example.com');
DELETE FROM email_change_requests WHERE user_id IN (SELECT id FROM users WHERE email = 'admin@example.com');
DELETE FROM users WHERE email = 'admin@example.com';
DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE code IN ('admin', 'user'));
DELETE FROM permissions WHERE code IN ('users.read', 'users.write', 'roles.manage', 'storage.upload');
DELETE FROM roles WHERE code IN ('admin', 'user');
