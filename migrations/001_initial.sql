PRAGMA foreign_keys = ON;

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_user_expiry ON sessions(user_id, expires_at);

CREATE TABLE studies (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    minimum_millicelsius INTEGER NOT NULL,
    maximum_millicelsius INTEGER NOT NULL,
    max_transit_seconds INTEGER NOT NULL,
    review_deadline_seconds INTEGER NOT NULL,
    business_timezone TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE sites (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    timezone TEXT NOT NULL,
    status TEXT NOT NULL,
    daily_limit INTEGER NOT NULL,
    cutoff_hour INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE sample_batches (
    id TEXT PRIMARY KEY,
    study_id TEXT NOT NULL REFERENCES studies(id),
    origin_site_id TEXT NOT NULL REFERENCES sites(id),
    external_ref TEXT NOT NULL,
    specimen_type TEXT NOT NULL,
    vial_count INTEGER NOT NULL,
    volume_ml INTEGER NOT NULL,
    state TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    shipment_id TEXT,
    quarantine_note TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(study_id, external_ref)
);
CREATE INDEX idx_sample_batches_state ON sample_batches(state, expires_at);
CREATE INDEX idx_sample_batches_site ON sample_batches(origin_site_id, created_at);

CREATE TABLE containers (
    id TEXT PRIMARY KEY,
    serial_number TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL,
    capacity_ml INTEGER NOT NULL,
    calibration_due_at TEXT NOT NULL,
    last_cleaned_at TEXT NOT NULL,
    reserved_shipment_id TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_containers_state_calibration ON containers(state, calibration_due_at);

CREATE TABLE shipments (
    id TEXT PRIMARY KEY,
    study_id TEXT NOT NULL REFERENCES studies(id),
    origin_site_id TEXT NOT NULL REFERENCES sites(id),
    destination_site_id TEXT NOT NULL REFERENCES sites(id),
    container_id TEXT NOT NULL REFERENCES containers(id),
    reference TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL,
    planned_dispatch_at TEXT NOT NULL,
    expected_arrival_at TEXT NOT NULL,
    dispatched_at TEXT,
    arrived_at TEXT,
    closed_at TEXT,
    total_volume_ml INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_shipments_route_window ON shipments(origin_site_id, destination_site_id, planned_dispatch_at);
CREATE INDEX idx_shipments_state ON shipments(state, expected_arrival_at);

CREATE TABLE shipment_items (
    shipment_id TEXT NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    batch_id TEXT NOT NULL UNIQUE REFERENCES sample_batches(id),
    added_at TEXT NOT NULL,
    PRIMARY KEY(shipment_id, batch_id)
);

CREATE TABLE custody_handoffs (
    id TEXT PRIMARY KEY,
    shipment_id TEXT NOT NULL REFERENCES shipments(id),
    from_custodian TEXT NOT NULL,
    to_custodian TEXT NOT NULL,
    location TEXT NOT NULL,
    status TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    resolved_at TEXT,
    resolution_note TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_handoff_pending_shipment ON custody_handoffs(shipment_id) WHERE status = 'pending';
CREATE INDEX idx_handoff_expiry ON custody_handoffs(status, expires_at);

CREATE TABLE temperature_readings (
    id TEXT PRIMARY KEY,
    shipment_id TEXT NOT NULL REFERENCES shipments(id),
    sensor_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    temperature_millicelsius INTEGER NOT NULL,
    recorded_at TEXT NOT NULL,
    received_at TEXT NOT NULL,
    UNIQUE(shipment_id, sensor_id, sequence)
);
CREATE INDEX idx_readings_shipment_time ON temperature_readings(shipment_id, recorded_at);

CREATE TABLE excursions (
    id TEXT PRIMARY KEY,
    shipment_id TEXT NOT NULL REFERENCES shipments(id),
    status TEXT NOT NULL,
    first_reading_at TEXT NOT NULL,
    last_reading_at TEXT NOT NULL,
    minimum_millicelsius INTEGER NOT NULL,
    maximum_millicelsius INTEGER NOT NULL,
    reading_count INTEGER NOT NULL,
    review_due_at TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_excursion_active_shipment ON excursions(shipment_id) WHERE status IN ('open', 'reviewing');
CREATE INDEX idx_excursion_review_due ON excursions(status, review_due_at);

CREATE TABLE review_decisions (
    id TEXT PRIMARY KEY,
    excursion_id TEXT NOT NULL REFERENCES excursions(id),
    reviewer TEXT NOT NULL,
    decision TEXT NOT NULL,
    rationale TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE audit_events (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    outcome TEXT NOT NULL,
    metadata_json TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_audit_entity ON audit_events(entity_type, entity_id, created_at);
CREATE INDEX idx_audit_request ON audit_events(request_id);

CREATE TABLE idempotency_records (
    scope TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    response_code INTEGER NOT NULL,
    response_body BLOB NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY(scope, idempotency_key)
);

CREATE TABLE outbox_jobs (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    payload BLOB NOT NULL,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL,
    available_at TEXT NOT NULL,
    locked_at TEXT,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_outbox_claim ON outbox_jobs(status, available_at, created_at);

CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);
