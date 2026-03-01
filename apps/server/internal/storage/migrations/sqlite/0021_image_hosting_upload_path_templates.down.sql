UPDATE system_configs
SET
	config_value_json = json_remove(
		CASE
			WHEN config_value_json IS NULL OR trim(config_value_json) = '' THEN '{}'
			ELSE config_value_json
		END,
		'$.local.uploadPathTemplate',
		'$.cloudflareR2.uploadPathTemplate',
		'$.aliyunOss.uploadPathTemplate'
	),
	updated_at = CURRENT_TIMESTAMP
WHERE config_key = 'image-hosting';
