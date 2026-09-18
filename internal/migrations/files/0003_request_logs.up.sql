CREATE TABLE request_logs (
    id            BIGSERIAL PRIMARY KEY,
    request_id    TEXT NOT NULL,
    method        TEXT NOT NULL,
    path          TEXT NOT NULL,
    status_code   INT NOT NULL,
    request_body  TEXT,
    response_body TEXT,
    remote_addr   TEXT,
    duration_ms   BIGINT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_request_logs_request_id ON request_logs (request_id);
CREATE INDEX idx_request_logs_created_at ON request_logs (created_at);
