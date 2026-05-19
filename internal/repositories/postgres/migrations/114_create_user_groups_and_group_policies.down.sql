-- +goose Down
DROP TABLE IF EXISTS group_policies;
DROP TABLE IF EXISTS user_groups;
DROP TABLE IF EXISTS `groups`;