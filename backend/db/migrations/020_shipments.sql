-- ============================================================
-- ShipAny shipments table + per-order delivery selection columns
-- ============================================================

-- Carrier/service the customer chose at checkout, plus optional pickup point
-- for locker / convenience-store services. These are persisted on the order
-- so the admin can later create a shipment with the right service.
ALTER TABLE orders
    ADD COLUMN selected_carrier    VARCHAR(64),
    ADD COLUMN selected_service    VARCHAR(64),
    ADD COLUMN pickup_point_id     VARCHAR(128),
    ADD COLUMN pickup_point_label  TEXT;

-- One shipment per order in v1 (the table allows multiple rows but the UI
-- assumes one). Status mirrors ShipAny's tracking lifecycle, normalised
-- to a small set:
--   created → pickup_requested → in_transit → delivered
--                                           ↘ exception
CREATE TABLE shipments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    shipany_shipment_id VARCHAR(128) NOT NULL,
    tracking_number     VARCHAR(128),
    tracking_url        TEXT,
    label_url           TEXT,
    carrier             VARCHAR(64) NOT NULL,
    service             VARCHAR(64) NOT NULL,
    fee_hkd             NUMERIC(10, 2) NOT NULL DEFAULT 0,
    status              VARCHAR(32) NOT NULL DEFAULT 'created',
    last_tracking_event JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shipments_order_id ON shipments(order_id);
CREATE UNIQUE INDEX idx_shipments_tracking
    ON shipments(tracking_number) WHERE tracking_number IS NOT NULL;
CREATE UNIQUE INDEX idx_shipments_shipany_id ON shipments(shipany_shipment_id);

CREATE TRIGGER trg_shipments_updated_at
    BEFORE UPDATE ON shipments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
