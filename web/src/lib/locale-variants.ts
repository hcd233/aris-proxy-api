import en from "../locales/en.json";
import zh from "../locales/zh.json";
import ja from "../locales/ja.json";

/* Locale-Stable labels: given a rendered translation (in any locale), return
 * every locale's text for it, so a rigid element can reserve the widest one
 * and keep its size when the language changes. A text shared by several keys
 * (e.g. "Retry") yields the union of all of them — the reserve may be a bit
 * wider than needed, but never narrower. Relative imports: vitest has no `@/`
 * alias. */

const TABLES: Record<string, string>[] = [en, zh, ja];

let index: Map<string, string[]> | null = null;

function buildIndex(): Map<string, string[]> {
  // Group texts that are linked through any key (union-find over texts): if
  // "Create Key" and "Create API Key" share the zh "创建密钥", all three
  // locales' texts of both keys form one group, so every member reserves the
  // same width and switching among them can never resize the element.
  const parent = new Map<string, string>();
  const find = (t: string): string => {
    let root = t;
    while (parent.get(root) !== root) root = parent.get(root)!;
    parent.set(t, root);
    return root;
  };
  const keys = Object.keys(en);
  for (const key of keys) {
    const texts = TABLES.map((table) => table[key] ?? en[key as keyof typeof en]);
    for (const t of texts) if (!parent.has(t)) parent.set(t, t);
    for (const t of texts.slice(1)) parent.set(find(t), find(texts[0]));
  }
  const groups = new Map<string, string[]>();
  for (const t of parent.keys()) {
    const root = find(t);
    const group = groups.get(root);
    if (group) group.push(t);
    else groups.set(root, [t]);
  }
  const out = new Map<string, string[]>();
  for (const group of groups.values()) {
    if (group.length > 1) for (const t of group) out.set(t, group);
  }
  return out;
}

/** All locales' texts for a translation; null when it is not one, or never changes. */
export function localeVariants(text: string): string[] | null {
  index ??= buildIndex();
  return index.get(text) ?? null;
}
