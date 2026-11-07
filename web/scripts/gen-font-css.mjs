// 从 @chinese-fonts/maple-mono-cn 生成指向 R2 的 @font-face CSS。
//
// 包里的 result.css 用相对路径 url("./<hash>.woff2")，构建时会被 Next 复制进
// _next/static/media 并 embed 进 Go 二进制（18 MB）。这里把它们改写成 R2 上的
// 绝对地址，产物只保留 CSS 声明；字体文件由 script/upload-fonts-r2.sh 上传。
//
// 用法：node scripts/gen-font-css.mjs [baseUrl]
// 包升级后重跑一次并提交 web/src/styles/maple-mono-cn.css。
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

const require = createRequire(import.meta.url);
const PKG = "@chinese-fonts/maple-mono-cn";
const DEFAULT_BASE = "https://static.lvlvko.top";

const base = (process.argv[2] || process.env.MAPLE_FONT_BASE_URL || DEFAULT_BASE).replace(
  /\/+$/,
  "",
);
const distDir = join(dirname(require.resolve(`${PKG}/package.json`)), "dist");
const outFile = new URL("../src/styles/maple-mono-cn.css", import.meta.url);

const weights = [
  { dir: "MapleMono-CN-Regular", slug: "regular" },
  { dir: "MapleMono-CN-Bold", slug: "bold" },
];

const sections = weights.map(({ dir, slug }) => {
  const source = readFileSync(join(distDir, dir, "result.css"), "utf8");
  const rewritten = source.replace(
    /url\("(?:\.\/)?([^"/]+\.woff2)"\)/g,
    (_match, file) => `url("${base}/fonts/maple-mono-cn/${slug}/${file}")`,
  );
  if (rewritten.includes('url("./')) {
    throw new Error(`${dir}/result.css 仍有相对 url()，请在 gen-font-css.mjs 里补规则`);
  }
  return rewritten.trim();
});

const header = `/* 由 scripts/gen-font-css.mjs 生成，请勿手改。字体文件托管在 ${base}，上传见 script/upload-fonts-r2.sh。 */\n`;

mkdirSync(new URL("../src/styles", import.meta.url), { recursive: true });
writeFileSync(outFile, header + sections.join("\n"));

const faces = sections.join("").match(/@font-face/g)?.length ?? 0;
console.log(`wrote src/styles/maple-mono-cn.css: ${faces} faces -> ${base}`);
