CREATE TABLE users (
    id serial primary key,
    name varchar not null,
    created_at timestamp not null default NOW(),
    updated_at timestamp not null default NOW()
);

CREATE OR REPLACE function update_updated_at()
RETURNS TRIGGER AS
$$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER trigger_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at();