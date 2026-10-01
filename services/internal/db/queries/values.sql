-- name: ListValues :many
-- Values in key order. A null period returns every period.
SELECT id, key, label, amount, unit, period, version, updated_at, updated_by
FROM source_values
WHERE sqlc.narg(period)::text IS NULL OR period = sqlc.narg(period)::text
ORDER BY key;
