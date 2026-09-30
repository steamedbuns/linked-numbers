-- The six tables from the spec's data model (docs/adr/0005). Kinds, units
-- and signs are text/smallint with CHECKs rather than enum types, so later
-- migrations can change them with a plain ALTER.

-- +goose Up
CREATE TABLE users (
    id           text PRIMARY KEY CHECK (id ~ '^u_[a-z0-9_]+$'),
    display_name text NOT NULL CHECK (btrim(display_name) <> ''),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE source_values (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key        text NOT NULL UNIQUE CHECK (key ~ '^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$'),
    label      text NOT NULL CHECK (btrim(label) <> ''),
    amount     numeric(20, 4) NOT NULL,
    unit       text NOT NULL CHECK (unit IN ('USD', '%', 'count')),
    period     text NOT NULL CHECK (btrim(period) <> ''),
    version    integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by text NOT NULL REFERENCES users (id)
);
CREATE INDEX source_values_period_idx ON source_values (period);

-- One row per edit; version is the value's version after the edit. value_id
-- has no foreign key: history outlives a deleted value.
CREATE TABLE value_changes (
    id         bigserial PRIMARY KEY,
    value_id   uuid NOT NULL,
    old_amount numeric(20, 4) NOT NULL,
    new_amount numeric(20, 4) NOT NULL,
    version    integer NOT NULL CHECK (version >= 2),
    reason     text NOT NULL CHECK (btrim(reason) <> ''),
    changed_by text NOT NULL REFERENCES users (id),
    changed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (value_id, version)
);
CREATE INDEX value_changes_value_id_id_idx ON value_changes (value_id, id);

-- Append-only. Grants alone don't stop a superuser or the table owner, so a
-- trigger rejects UPDATE, DELETE and TRUNCATE for everyone.
REVOKE UPDATE, DELETE, TRUNCATE ON value_changes FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION value_changes_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'value_changes is append-only: % is not allowed', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER value_changes_no_update_delete
    BEFORE UPDATE OR DELETE ON value_changes
    FOR EACH ROW EXECUTE FUNCTION value_changes_append_only();
CREATE TRIGGER value_changes_no_truncate
    BEFORE TRUNCATE ON value_changes
    FOR EACH STATEMENT EXECUTE FUNCTION value_changes_append_only();

CREATE TABLE reports (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title      text NOT NULL CHECK (btrim(title) <> ''),
    created_by text NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- The position constraint is deferrable so a reorder can swap positions
-- inside one transaction (SET CONSTRAINTS ... DEFERRED).
CREATE TABLE report_cells (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id uuid NOT NULL REFERENCES reports (id) ON DELETE CASCADE,
    position  integer NOT NULL CHECK (position >= 0),
    kind      text NOT NULL CHECK (kind IN ('text', 'value_ref', 'sum')),
    text      text,
    value_id  uuid REFERENCES source_values (id) ON DELETE RESTRICT,
    label     text,
    CONSTRAINT report_cells_position_key UNIQUE (report_id, position) DEFERRABLE INITIALLY IMMEDIATE,
    CONSTRAINT report_cells_kind_fields CHECK (
        (kind = 'text' AND text IS NOT NULL AND value_id IS NULL)
        OR (kind = 'value_ref' AND text IS NULL AND value_id IS NOT NULL)
        OR (kind = 'sum' AND text IS NULL AND value_id IS NULL AND label IS NOT NULL)
    )
);
CREATE INDEX report_cells_value_id_idx ON report_cells (value_id) WHERE value_id IS NOT NULL;

-- A sum cell's inputs. That every sum cell has at least one input, and that
-- sum_cell_id names a sum cell, is checked by values-api (LN-3.4).
CREATE TABLE report_cell_sum_inputs (
    sum_cell_id uuid NOT NULL REFERENCES report_cells (id) ON DELETE CASCADE,
    value_id    uuid NOT NULL REFERENCES source_values (id) ON DELETE RESTRICT,
    sign        smallint NOT NULL CHECK (sign IN (-1, 1)),
    PRIMARY KEY (sum_cell_id, value_id)
);
CREATE INDEX report_cell_sum_inputs_value_id_idx ON report_cell_sum_inputs (value_id);

-- +goose Down
DROP TABLE report_cell_sum_inputs;
DROP TABLE report_cells;
DROP TABLE reports;
DROP TABLE value_changes;
DROP FUNCTION value_changes_append_only();
DROP TABLE source_values;
DROP TABLE users;
