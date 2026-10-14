-- loadtest-cleanup.sql 清理压测数据（按 api_key_id 精确定向）
--
-- 用法：
--   psql "$PROD_DATABASE_URL" -v api_key_id=<压测专用 Key 的 ID> -f scripts/loadtest-cleanup.sql
--
-- 幂等：重复执行第二次删 0 行。
-- 安全：所有删除都带 api_key_id 精确条件；孤儿判定用 jsonb_array_elements_text 展开
--       message_ids / tool_ids 数组后与「仍被活跃 session 引用的 ID」求差集，
--       等价 SoftDeletePurge 的算法，只是按 api_key_id 定向。
--
-- 仅预览不删除：注释掉 "Step 2~6" 之间的语句后执行。

\set ON_ERROR_STOP on

-- ========== Step 1 预览核对（先看再删） ==========
SELECT 'audits_to_delete' AS what, count(*) AS n
FROM model_call_audits WHERE api_key_id = CAST(:'api_key_id' AS bigint)
UNION ALL
SELECT 'sessions_to_delete', count(*)
FROM sessions WHERE api_key_id = CAST(:'api_key_id' AS bigint) AND deleted_at = 0;

BEGIN;

-- ========== Step 2 删审计 ==========
DELETE FROM model_call_audits WHERE api_key_id = CAST(:'api_key_id' AS bigint);

-- ========== Step 3 收集压测 session 引用的 message/tool ID ==========
CREATE TEMP TABLE lt_msg AS
SELECT DISTINCT x::bigint AS id
FROM sessions s, jsonb_array_elements_text(s.message_ids::jsonb) x
WHERE s.api_key_id = CAST(:'api_key_id' AS bigint);

CREATE TEMP TABLE lt_tool AS
SELECT DISTINCT x::bigint AS id
FROM sessions s, jsonb_array_elements_text(s.tool_ids::jsonb) x
WHERE s.api_key_id = CAST(:'api_key_id' AS bigint);

-- ========== Step 4 软删压测 session（deleted_at=0 为活跃） ==========
UPDATE sessions SET deleted_at = extract(epoch from now())::bigint
WHERE api_key_id = CAST(:'api_key_id' AS bigint) AND deleted_at = 0;

-- ========== Step 5 硬删孤儿 message/tool ==========
-- 孤儿 = 压测引用过、且不再被任何活跃 session 引用
CREATE TEMP TABLE still_used_msg AS
SELECT DISTINCT x::bigint AS id
FROM sessions s, jsonb_array_elements_text(s.message_ids::jsonb) x
WHERE s.deleted_at = 0;

CREATE TEMP TABLE still_used_tool AS
SELECT DISTINCT x::bigint AS id
FROM sessions s, jsonb_array_elements_text(s.tool_ids::jsonb) x
WHERE s.deleted_at = 0;

DELETE FROM messages
WHERE id IN (SELECT id FROM lt_msg)
  AND id NOT IN (SELECT id FROM still_used_msg);

DELETE FROM tools
WHERE id IN (SELECT id FROM lt_tool)
  AND id NOT IN (SELECT id FROM still_used_tool);

-- ========== Step 6 硬删已软删的压测 session ==========
DELETE FROM sessions WHERE api_key_id = CAST(:'api_key_id' AS bigint) AND deleted_at <> 0;

COMMIT;

-- ========== Step 7 复核（应全部为 0） ==========
SELECT 'audits_left' AS what, count(*) AS n
FROM model_call_audits WHERE api_key_id = CAST(:'api_key_id' AS bigint)
UNION ALL
SELECT 'sessions_left', count(*)
FROM sessions WHERE api_key_id = CAST(:'api_key_id' AS bigint);
