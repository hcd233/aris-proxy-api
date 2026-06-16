-- 回填存量 session 的 models 列
-- 说明：从 message_ids 聚合 assistant 消息的 model 字段（非空），去重后写回 sessions.models
-- 注意：该 SQL 一次性处理全表，数据量大时建议分批或低峰期执行；执行前请备份。

UPDATE sessions s
SET models = sub.models
FROM (
    SELECT
        s.id,
        COALESCE(
            jsonb_agg(DISTINCT m.model ORDER BY m.model) FILTER (WHERE m.model IS NOT NULL AND m.model <> ''),
            '[]'::jsonb
        ) AS models
    FROM sessions s
    CROSS JOIN LATERAL jsonb_array_elements_text(s.message_ids::jsonb) AS mid
    JOIN messages m ON m.id = mid::bigint
    WHERE s.deleted_at = 0
      AND m.message::jsonb ->> 'role' = 'assistant'
    GROUP BY s.id
) sub
WHERE s.id = sub.id
  AND (s.models IS NULL OR s.models = '[]'::jsonb);
