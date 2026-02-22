ALTER TABLE spaces DROP CONSTRAINT IF EXISTS fk_spaces_category_id;

DROP INDEX IF EXISTS idx_spaces_category_id;
DROP INDEX IF EXISTS idx_space_categories_is_default;

ALTER TABLE spaces DROP COLUMN IF EXISTS category_id;

DROP TABLE IF EXISTS space_categories;
