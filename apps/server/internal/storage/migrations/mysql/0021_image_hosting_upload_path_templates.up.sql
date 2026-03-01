UPDATE system_configs
SET
	config_value_json = JSON_SET(
		JSON_SET(
			JSON_SET(
				CASE
					WHEN JSON_VALID(config_value_json) THEN CAST(config_value_json AS JSON)
					ELSE JSON_OBJECT()
				END,
				'$.local.uploadPathTemplate',
				COALESCE(
					NULLIF(JSON_UNQUOTE(JSON_EXTRACT(config_value_json, '$.local.uploadPathTemplate')), ''),
					'images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}'
				)
			),
			'$.cloudflareR2.uploadPathTemplate',
			COALESCE(
				NULLIF(JSON_UNQUOTE(JSON_EXTRACT(config_value_json, '$.cloudflareR2.uploadPathTemplate')), ''),
				'images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}'
			)
		),
		'$.aliyunOss.uploadPathTemplate',
		COALESCE(
			NULLIF(JSON_UNQUOTE(JSON_EXTRACT(config_value_json, '$.aliyunOss.uploadPathTemplate')), ''),
			'images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}'
		)
	),
	updated_at = UTC_TIMESTAMP(6)
WHERE config_key = 'image-hosting';
