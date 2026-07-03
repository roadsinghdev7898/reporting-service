CREATE TABLE IF NOT EXISTS report_templates (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(120) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    base_table VARCHAR(200) NOT NULL,
    base_alias VARCHAR(64) NOT NULL,
    default_sort JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_group_by JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS report_columns (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES report_templates(id) ON DELETE CASCADE,
    column_key VARCHAR(120) NOT NULL,
    expression TEXT NOT NULL,
    alias VARCHAR(120) NOT NULL,
    data_type VARCHAR(50) NOT NULL DEFAULT 'text',
    is_visible BOOLEAN NOT NULL DEFAULT TRUE,
    is_sortable BOOLEAN NOT NULL DEFAULT FALSE,
    is_exportable BOOLEAN NOT NULL DEFAULT TRUE,
    position INT NOT NULL DEFAULT 0,
    UNIQUE (template_id, column_key),
    UNIQUE (template_id, alias)
);

CREATE TABLE IF NOT EXISTS report_joins (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES report_templates(id) ON DELETE CASCADE,
    join_type VARCHAR(20) NOT NULL CHECK (join_type IN ('INNER', 'LEFT', 'RIGHT', 'FULL')),
    table_name VARCHAR(200) NOT NULL,
    table_alias VARCHAR(64) NOT NULL,
    on_expression TEXT NOT NULL,
    position INT NOT NULL DEFAULT 0,
    UNIQUE (template_id, table_alias)
);

CREATE TABLE IF NOT EXISTS report_filters (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES report_templates(id) ON DELETE CASCADE,
    filter_key VARCHAR(120) NOT NULL,
    expression TEXT NOT NULL,
    data_type VARCHAR(50) NOT NULL DEFAULT 'text',
    operators JSONB NOT NULL,
    is_required BOOLEAN NOT NULL DEFAULT FALSE,
    is_multi_value BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NOT NULL DEFAULT '',
    UNIQUE (template_id, filter_key)
);

CREATE INDEX IF NOT EXISTS idx_report_templates_code_active ON report_templates(code) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_report_columns_template_position ON report_columns(template_id, position);
CREATE INDEX IF NOT EXISTS idx_report_joins_template_position ON report_joins(template_id, position);
CREATE INDEX IF NOT EXISTS idx_report_filters_template_key ON report_filters(template_id, filter_key);

INSERT INTO report_templates (code, name, description, base_table, base_alias, default_sort)
VALUES (
    'submission_summary',
    'Submission Summary',
    'Example multi-table report template for assignment submissions.',
    'submissions',
    's',
    '[{"key":"submitted_at","direction":"desc"}]'::jsonb
)
ON CONFLICT (code) DO NOTHING;

INSERT INTO report_columns (template_id, column_key, expression, alias, data_type, is_visible, is_sortable, position)
SELECT t.id, c.column_key, c.expression, c.alias, c.data_type, c.is_visible, c.is_sortable, c.position
FROM report_templates t
CROSS JOIN (VALUES
    ('submission_id', 's.id', 'submission_id', 'uuid', true, true, 1),
    ('student_name', 'u.full_name', 'student_name', 'text', true, true, 2),
    ('assignment_title', 'a.title', 'assignment_title', 'text', true, true, 3),
    ('status', 's.status', 'status', 'text', true, true, 4),
    ('score', 'e.score', 'score', 'number', true, true, 5),
    ('submitted_at', 's.submitted_at', 'submitted_at', 'timestamp', true, true, 6)
) AS c(column_key, expression, alias, data_type, is_visible, is_sortable, position)
WHERE t.code = 'submission_summary'
ON CONFLICT (template_id, column_key) DO NOTHING;

INSERT INTO report_joins (template_id, join_type, table_name, table_alias, on_expression, position)
SELECT t.id, j.join_type, j.table_name, j.table_alias, j.on_expression, j.position
FROM report_templates t
CROSS JOIN (VALUES
    ('LEFT', 'users', 'u', 'u.id = s.student_id', 1),
    ('LEFT', 'assignments', 'a', 'a.id = s.assignment_id', 2),
    ('LEFT', 'evaluations', 'e', 'e.submission_id = s.id', 3)
) AS j(join_type, table_name, table_alias, on_expression, position)
WHERE t.code = 'submission_summary'
ON CONFLICT (template_id, table_alias) DO NOTHING;

INSERT INTO report_filters (template_id, filter_key, expression, data_type, operators, is_multi_value, description)
SELECT t.id, f.filter_key, f.expression, f.data_type, f.operators::jsonb, f.is_multi_value, f.description
FROM report_templates t
CROSS JOIN (VALUES
    ('status', 's.status', 'text', '["equals","in"]', true, 'Filter by submission status.'),
    ('student_name', 'u.full_name', 'text', '["contains","starts_with","equals"]', false, 'Filter by student name.'),
    ('score', 'e.score', 'number', '["between","greater_than_or_equal","less_than_or_equal"]', false, 'Filter by evaluation score.'),
    ('submitted_at', 's.submitted_at', 'timestamp', '["between","greater_than","less_than"]', false, 'Filter by submission timestamp.')
) AS f(filter_key, expression, data_type, operators, is_multi_value, description)
WHERE t.code = 'submission_summary'
ON CONFLICT (template_id, filter_key) DO NOTHING;
