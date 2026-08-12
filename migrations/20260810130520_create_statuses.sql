-- +goose Up
CREATE TABLE statuses (
    id SMALLINT PRIMARY KEY,
    code VARCHAR(32) NOT NULL,
    name VARCHAR(64) NOT NULL,
    description TEXT,

    CONSTRAINT statuses_code_unique UNIQUE (code)
);

INSERT INTO statuses (id, code, name, description)
VALUES
    (1, 'ACTIVE', 'Active', 'Data is active'),
    (2, 'INACTIVE', 'Inactive', 'Data is inactive'),
    (3, 'PENDING', 'Pending', 'Data is pending'),
    (4, 'SUSPENDED', 'Suspended', 'Data is suspended'),
    (5, 'BLOCKED', 'Blocked', 'Data is blocked');

-- +goose Down
DROP TABLE IF EXISTS statuses;
