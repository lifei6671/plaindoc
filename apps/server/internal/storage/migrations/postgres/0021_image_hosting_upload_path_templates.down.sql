UPDATE system_configs
SET
	config_value_json = (
		(
			(
				CASE
					WHEN config_value_json IS NULL OR btrim(config_value_json) = '' THEN '{}'::jsonb
					ELSE config_value_json::jsonb
				END
			) #- '{local,uploadPathTemplate}'
		) #- '{cloudflareR2,uploadPathTemplate}'
	) #- '{aliyunOss,uploadPathTemplate}'
)::text,
	updated_at = NOW()
WHERE config_key = 'image-hosting';
