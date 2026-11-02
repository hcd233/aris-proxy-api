function isPlainObject(v: unknown): v is Record<string, unknown> {
  if (v === null || typeof v !== "object") return false;
  const proto = Object.getPrototypeOf(v);
  return proto === Object.prototype || proto === null;
}

/**
 * 结构共享：返回与 next 深相等的值，但尽量复用 prev 中相等的子树引用。
 * 整体相等时直接返回 prev，从而让依赖引用比较的下游（useMemo、recharts 的
 * 动画触发）在刷新拿到相同数据时保持静止，只有真正变化的部分换新引用。
 * 只处理 JSON 形态的数据（普通对象、数组、原始值）。
 */
export function replaceEqualDeep<T>(prev: unknown, next: T): T {
  if (Object.is(prev, next)) return prev as T;

  const prevIsArray = Array.isArray(prev);
  const nextIsArray = Array.isArray(next);
  if (prevIsArray && nextIsArray) {
    const p = prev as unknown[];
    const n = next as unknown[];
    let same = p.length === n.length;
    const out = n.map((item, i) => {
      const shared = replaceEqualDeep(p[i], item);
      if (shared !== p[i]) same = false;
      return shared;
    });
    return (same ? p : out) as T;
  }

  if (!prevIsArray && !nextIsArray && isPlainObject(prev) && isPlainObject(next)) {
    const pKeys = Object.keys(prev);
    const nKeys = Object.keys(next);
    let same = pKeys.length === nKeys.length;
    const out: Record<string, unknown> = {};
    for (const k of nKeys) {
      const shared = replaceEqualDeep(prev[k], next[k]);
      out[k] = shared;
      if (shared !== prev[k] || !Object.hasOwn(prev, k)) same = false;
    }
    return (same ? prev : out) as T;
  }

  return next;
}
