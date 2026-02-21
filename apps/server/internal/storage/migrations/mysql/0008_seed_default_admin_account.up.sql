-- 中文注释：仅在首次初始化（users 为空）时注入默认管理员账号。
INSERT INTO users (
	user_id,
	email,
	password_hash,
	name,
	status
)
SELECT
	'01k5aa0bb1cc2dd3ee4ff5gg6h',
	'admin@iminho.me',
	'$2b$10$wJuVpT7IG1V9w6JAluIyaeG0TDiRQLagPWcKK54m59NJWhiTIl.mu',
	'Admin',
	'active'
FROM DUAL
WHERE NOT EXISTS (
	SELECT 1 FROM users LIMIT 1
);

-- 中文注释：首次初始化成功后，授予默认账号 platform_admin 权限。
INSERT INTO user_admin_roles (
	user_id,
	role
)
SELECT
	'01k5aa0bb1cc2dd3ee4ff5gg6h',
	'platform_admin'
FROM DUAL
WHERE EXISTS (
	SELECT 1
	FROM users
	WHERE user_id = '01k5aa0bb1cc2dd3ee4ff5gg6h'
	  AND email = 'admin@iminho.me'
)
AND (SELECT COUNT(1) FROM users) = 1
AND NOT EXISTS (
	SELECT 1
	FROM user_admin_roles
	WHERE user_id = '01k5aa0bb1cc2dd3ee4ff5gg6h'
	  AND role = 'platform_admin'
);
