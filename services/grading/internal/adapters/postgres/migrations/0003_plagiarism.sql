ALTER TABLE risks DROP CONSTRAINT risks_category_check;
ALTER TABLE risks ADD CONSTRAINT risks_category_check CHECK
 ((category='risk.behavior_anomaly' AND target_type='attempt') OR (category='risk.plagiarism' AND target_type='submission'));
CREATE INDEX risks_category_id_idx ON risks(category,id);
