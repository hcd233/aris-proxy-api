// cronstrue 及其 locale 是带副作用的导入，单独成模块，避免经 lib/utils 进入所有页面的包
import cronstrue from "cronstrue";
import "cronstrue/locales/zh_CN";

/** cron 表达式转自然语言；解析失败原样返回表达式。 */
export function specToHuman(spec: string, locale: string): string {
  try {
    const cronLocale = locale === "zh" ? "zh_CN" : "en";
    return cronstrue.toString(spec, { locale: cronLocale });
  } catch {
    return spec;
  }
}

/** 仅接受 5 段标准 cron 且能被 cronstrue 解析的表达式。 */
export function isValidCronSpec(spec: string): boolean {
  if (!spec || spec.trim().split(/\s+/).length !== 5) return false;
  try {
    cronstrue.toString(spec);
    return true;
  } catch {
    return false;
  }
}
