DROP INDEX IF EXISTS idx_spaces_category;

ALTER TABLE spaces DROP COLUMN IF EXISTS category;
