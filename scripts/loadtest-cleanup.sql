-- loadtest-cleanup.sql 清理压测数据（按压测专用 api_key_id 精确定向）
--
-- 用法：
--   psql "$PROD_DATABASE_URL" \
--     -v api_key_id=<专用压测 Key 的 ID> \
--     -v confirm=delete-loadtest-data \
--     -f scripts/loadtest-cleanup.sql
--
-- 仅预览不删除（仍会打印完整删除量）：
--   ... 同上追加 -v dry_run=1
--
-- 安全门槛（任一不满足立即以非 0 退出，不执行任何写操作）：
--   1. -v api_key_id 必须指向「存在且未软删」的 Key；
--   2. 该 Key 的 name 必须以 loadtest- 开头（专用压测 Key 命名约定）
--      —— 防止误传主 Key（id=1）或真实用户 Key，导致真实 session/message/tool
--      被软删 + 硬删（不可逆）；
--   3. -v confirm=delete-loadtest-data 二次确认。
--
-- 幂等：重复执行第二次删 0 行。
-- 孤儿判定：删除「压测 session 引用过、且不被任何留存 session 引用」的 message/tool，
--           等价 SoftDeletePurge 算法，只是候选集限定在该 api_key_id 的 session 内；
--           留存集合按「活跃 且 不属于目标 Key」计算，与软删压测 session 后的真实状态一致。

\set ON_ERROR_STOP on
SET client_min_messages = warning; -- 静音 DROP TABLE IF EXISTS 的 not-exist NOTICE

-- ========== Step 0 参数守卫 ==========
\if :{?api_key_id}
\else
DO $$ BEGIN RAISE EXCEPTION 'REFUSED: 必须用 -v api_key_id=<专用压测 Key ID> 指定目标'; END $$;
\endif

\if :{?confirm}
\else
DO $$ BEGIN RAISE EXCEPTION 'REFUSED: 必须用 -v confirm=delete-loadtest-data 二次确认'; END $$;
\endif

-- count 聚合保证恒有一行（0 行会让 \gset 以 "no rows returned" 中止，信息不达意）
SELECT count(*) AS lt_key_found, coalesce(max(k.name), '') AS lt_key_name
FROM proxy_api_keys k
WHERE k.id = CAST(:'api_key_id' AS bigint) AND k.deleted_at = 0
\gset

\if :lt_key_found
\else
DO $$ BEGIN RAISE EXCEPTION 'REFUSED: api_key_id 不存在或已软删'; END $$;
\endif

SELECT (:'lt_key_name' LIKE 'loadtest-%') AS lt_key_is_dedicated
\gset

\if :lt_key_is_dedicated
\else
DO $$ BEGIN RAISE EXCEPTION 'REFUSED: api_key_id 不是专用压测 Key（name 需以 loadtest- 开头）'; END $$;
\endif

\echo '目标 Key:' :lt_key_name

-- ========== Step 1 候选集（预览与删除共用同一套集合） ==========
DROP TABLE IF EXISTS lt_msg, lt_tool, lt_keep_msg, lt_keep_tool;

-- 压测 session 引用过的 message/tool（候选删除集）
CREATE TEMP TABLE lt_msg AS
SELECT DISTINCT x::bigint AS id
FROM sessions s, jsonb_array_elements_text(s.message_ids::jsonb) x
WHERE s.api_key_id = CAST(:'api_key_id' AS bigint);

CREATE TEMP TABLE lt_tool AS
SELECT DISTINCT x::bigint AS id
FROM sessions s, jsonb_array_elements_text(s.tool_ids::jsonb) x
WHERE s.api_key_id = CAST(:'api_key_id' AS bigint);

-- 仍被留存 session 引用的 id（活跃 且 不属于目标 Key）
CREATE TEMP TABLE lt_keep_msg AS
SELECT DISTINCT x::bigint AS id
FROM sessions s, jsonb_array_elements_text(s.message_ids::jsonb) x
WHERE s.deleted_at = 0 AND s.api_key_id <> CAST(:'api_key_id' AS bigint);

CREATE TEMP TABLE lt_keep_tool AS
SELECT DISTINCT x::bigint AS id
FROM sessions s, jsonb_array_elements_text(s.tool_ids::jsonb) x
WHERE s.deleted_at = 0 AND s.api_key_id <> CAST(:'api_key_id' AS bigint);

-- ========== Step 2 预览（先看再删） ==========
SELECT 'audits_to_delete' AS what, count(*) AS n
FROM model_call_audits WHERE api_key_id = CAST(:'api_key_id' AS bigint)
UNION ALL
SELECT 'active_sessions_to_soft_delete', count(*)
FROM sessions WHERE api_key_id = CAST(:'api_key_id' AS bigint) AND deleted_at = 0
UNION ALL
SELECT 'soft_deleted_sessions_to_hard_delete', count(*)
FROM sessions WHERE api_key_id = CAST(:'api_key_id' AS bigint) AND deleted_at <> 0
UNION ALL
SELECT 'orphan_messages_to_delete', count(*)
FROM messages m
WHERE m.id IN (SELECT id FROM lt_msg)
  AND NOT EXISTS (SELECT 1 FROM lt_keep_msg k WHERE k.id = m.id)
UNION ALL
SELECT 'orphan_tools_to_delete', count(*)
FROM tools t
WHERE t.id IN (SELECT id FROM lt_tool)
  AND NOT EXISTS (SELECT 1 FROM lt_keep_tool k WHERE k.id = t.id);

\if :{?dry_run}
\echo '[DRY-RUN] 以上为完整预览（与真实删除同源），跳过写操作'
\else

BEGIN;
SET LOCAL statement_timeout = '5min';
SET LOCAL lock_timeout = '30s';

-- ========== Step 3 删审计 ==========
DELETE FROM model_call_audits WHERE api_key_id = CAST(:'api_key_id' AS bigint);

-- ========== Step 4 软删压测 session（deleted_at=0 为活跃） ==========
UPDATE sessions SET deleted_at = extract(epoch from now())::bigint
WHERE api_key_id = CAST(:'api_key_id' AS bigint) AND deleted_at = 0;

-- ========== Step 5 硬删孤儿 message/tool ==========
-- NOT EXISTS 而非 NOT IN：留存集合可能含 NULL（jsonb array 中的 JSON null），
-- NOT IN 遇 NULL 整条判定为 NULL，会静默一条不删却在复核期看不出异常。
DELETE FROM messages m
WHERE m.id IN (SELECT id FROM lt_msg)
  AND NOT EXISTS (SELECT 1 FROM lt_keep_msg k WHERE k.id = m.id);

DELETE FROM tools t
WHERE t.id IN (SELECT id FROM lt_tool)
  AND NOT EXISTS (SELECT 1 FROM lt_keep_tool k WHERE k.id = t.id);

-- ========== Step 6 硬删已软删的压测 session ==========
DELETE FROM sessions WHERE api_key_id = CAST(:'api_key_id' AS bigint) AND deleted_at <> 0;

COMMIT;

\endif

-- ========== Step 7 复核（应全部为 0；dry-run 下为原始存量） ==========
SELECT 'audits_left' AS what, count(*) AS n
FROM model_call_audits WHERE api_key_id = CAST(:'api_key_id' AS bigint)
UNION ALL
SELECT 'sessions_left', count(*)
FROM sessions WHERE api_key_id = CAST(:'api_key_id' AS bigint);
