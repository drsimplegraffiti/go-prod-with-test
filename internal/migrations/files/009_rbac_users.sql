ALTER TABLE users
ADD COLUMN role_id UUID;

UPDATE users u
SET role_id = r.id
FROM roles r
WHERE r.name = COALESCE(u.role, 'user');

ALTER TABLE users
ALTER COLUMN role_id SET NOT NULL;

ALTER TABLE users
ADD CONSTRAINT fk_users_role
FOREIGN KEY (role_id)
REFERENCES roles(id);

CREATE INDEX IF NOT EXISTS idx_users_role_id
ON users(role_id);

ALTER TABLE users
DROP COLUMN role;
