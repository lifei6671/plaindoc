UPDATE system_configs
SET
	config_value_json = (
		jsonb_set(
			jsonb_set(
				jsonb_set(
					CASE
						WHEN config_value_json IS NULL OR btrim(config_value_json) = '' THEN '{}'::jsonb
						ELSE config_value_json::jsonb
					END,
					'{local,uploadPathTemplate}',
					to_jsonb(
						COALESCE(
							NULLIF(config_value_json::jsonb -> 'local' ->> 'uploadPathTemplate', ''),
							'images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}'
						)
					),
					true
				),
				'{cloudflareR2,uploadPathTemplate}',
				to_jsonb(
					COALESCE(
						NULLIF(config_value_json::jsonb -> 'cloudflareR2' ->> 'uploadPathTemplate', ''),
						'images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}'
					)
				),
				true
			),
			'{aliyunOss,uploadPathTemplate}',
			to_jsonb(
				COALESCE(
					NULLIF(config_value_json::jsonb -> 'aliyunOss' ->> 'uploadPathTemplate', ''),
					'images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}'
				)
			),
			true
		)
	)::text,
	updated_at = NOW()
WHERE config_key = 'image-hosting';
