UPDATE system_configs
SET
	config_value_json = json_set(
		json_set(
			json_set(
				CASE
					WHEN config_value_json IS NULL OR trim(config_value_json) = '' THEN '{}'
					ELSE config_value_json
				END,
				'$.local.uploadPathTemplate',
				COALESCE(
					NULLIF(json_extract(config_value_json, '$.local.uploadPathTemplate'), ''),
					'images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}'
				)
			),
			'$.cloudflareR2.uploadPathTemplate',
			COALESCE(
				NULLIF(json_extract(config_value_json, '$.cloudflareR2.uploadPathTemplate'), ''),
				'images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}'
			)
		),
		'$.aliyunOss.uploadPathTemplate',
		COALESCE(
			NULLIF(json_extract(config_value_json, '$.aliyunOss.uploadPathTemplate'), ''),
			'images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}'
		)
	),
	updated_at = CURRENT_TIMESTAMP
WHERE config_key = 'image-hosting';
