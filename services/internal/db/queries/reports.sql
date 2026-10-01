-- name: ListReports :many
SELECT id, title, created_by, created_at
FROM reports
ORDER BY created_at, id;

-- name: ListReportValueIDs :many
-- The distinct values a report uses, through value_ref cells or sum inputs.
SELECT c.value_id::uuid AS value_id
FROM report_cells c
WHERE c.report_id = @report_id AND c.value_id IS NOT NULL
UNION
SELECT i.value_id
FROM report_cell_sum_inputs i
JOIN report_cells c ON c.id = i.sum_cell_id
WHERE c.report_id = @report_id
ORDER BY value_id;
