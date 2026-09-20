CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (role_id, permission_id)
);

INSERT INTO roles (name, description)
VALUES
    ('user', 'Default application user'),
    ('admin', 'Application administrator')
ON CONFLICT (name) DO NOTHING;

INSERT INTO permissions (name, description)
VALUES
    ('post:read', 'Read posts'),
    ('post:create', 'Create posts'),
    ('post:update', 'Update posts'),
    ('post:delete', 'Delete posts'),

    ('user:read', 'Read users'),
    ('user:create', 'Create users'),
    ('user:update', 'Update users'),
    ('user:delete', 'Delete users'),

    ('job:read', 'Read jobs'),
    ('job:cancel', 'Cancel jobs'),

    ('role:read', 'Read roles'),
    ('role:create', 'Create roles'),
    ('role:update', 'Update roles'),
    ('role:delete', 'Delete roles'),

    ('role:permission:add', 'Add permission to role'),
    ('role:permission:remove', 'Remove permission from role'),

    ('user:role:update', 'Change user role')
ON CONFLICT (name) DO NOTHING;

-- Default user permissions.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'user'
AND p.name IN (
    'post:read',
    'post:create',
    'post:update',
    'post:delete',
    'job:read',
    'job:cancel'
)
ON CONFLICT DO NOTHING;

-- Admin gets everything.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;
