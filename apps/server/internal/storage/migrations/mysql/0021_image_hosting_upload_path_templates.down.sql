UPDATE system_configs
SET
	config_value_json = JSON_REMOVE(
		CAST(config_value_json AS JSON),
		'$.local.uploadPathTemplate',
		'$.cloudflareR2.uploadPathTemplate',
		'$.aliyunOss.uploadPathTemplate'
	),
	updated_at = UTC_TIMESTAMP(6)
WHERE config_key = 'image-hosting'
  AND JSON_VALID(config_value_json);
