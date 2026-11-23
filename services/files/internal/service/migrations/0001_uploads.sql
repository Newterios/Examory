CREATE TABLE uploads (
 id uuid PRIMARY KEY, owner_id uuid NOT NULL, group_id uuid, name text NOT NULL, mime text NOT NULL,
 size bigint NOT NULL CHECK(size BETWEEN 1 AND 2097152), expires_at timestamptz NOT NULL,
 sha256 text CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-f]{64}$'), completed_at timestamptz
);
