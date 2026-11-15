CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE projects (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX projects_owner_id_idx ON projects(owner_id);

CREATE TABLE issues (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_by uuid NOT NULL REFERENCES users(id),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'done')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX issues_project_id_created_at_idx ON issues(project_id, created_at DESC);

CREATE TABLE comments (
    id uuid PRIMARY KEY,
    issue_id uuid NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_id uuid NOT NULL REFERENCES users(id),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 4000),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX comments_issue_id_created_at_idx ON comments(issue_id, created_at);
