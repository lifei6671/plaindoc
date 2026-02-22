ALTER TABLE spaces DROP FOREIGN KEY fk_spaces_category_id;

DROP INDEX idx_spaces_category_id ON spaces;
DROP INDEX idx_space_categories_is_default ON space_categories;

ALTER TABLE spaces DROP COLUMN category_id;

DROP TABLE IF EXISTS space_categories;
