CREATE INDEX idx_samples_shipment_state ON sample_batches(shipment_id, state);
CREATE INDEX idx_shipments_study_created ON shipments(study_id, created_at);
CREATE INDEX idx_handoffs_custodians ON custody_handoffs(to_custodian, status, created_at);
CREATE INDEX idx_jobs_aggregate ON outbox_jobs(kind, aggregate_id, created_at);
