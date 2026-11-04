"use client";

import {
  CodeBlock,
  CodeBlockActions,
  CodeBlockContent,
  CodeBlockCopyButton,
  CodeBlockHeader,
  CodeBlockLine,
  CodeBlockTitle,
} from "@/components/ui/code-block";
import { useHighlightedCode } from "@/hooks/use-highlighted-code";
import { shouldHighlight } from "@/lib/highlighter";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface HighlightedCodeProps {
  code: string;
  language: string;
  /** 头部标题（通常是语言名或「输入 / 输出」）；不传则复制按钮浮在右上角 */
  title?: string;
  size?: "default" | "sm";
  /** 作用在 CodeBlockContent（滚动区）上，如 max-h-64 */
  className?: string;
  /** 作用在 CodeBlock 外框上，如 my-3 */
  containerClassName?: string;
  lineNumbers?: boolean;
  wrap?: boolean;
}

/**
 * shadcn Code Block + shiki 的项目组合：按需加载语言、逐行高亮（流式时只分词新行），
 * 超大内容（> MAX_HIGHLIGHT_CHARS）与未收录语言退化为纯文本行。
 */
export function HighlightedCode({
  code,
  language,
  title,
  size = "default",
  className,
  containerClassName,
  lineNumbers,
  wrap,
}: HighlightedCodeProps) {
  const t = useT();
  const lines = useHighlightedCode(code, shouldHighlight(code) ? language : "");
  const copy = (
    <CodeBlockActions>
      <CodeBlockCopyButton value={code} ariaLabel={t("markdown.copy_code_aria")} />
    </CodeBlockActions>
  );

  return (
    <CodeBlock size={size} className={containerClassName}>
      {title ? (
        <CodeBlockHeader>
          <CodeBlockTitle className="font-mono">{title}</CodeBlockTitle>
          {copy}
        </CodeBlockHeader>
      ) : (
        copy
      )}
      <CodeBlockContent lineNumbers={lineNumbers} wrap={wrap} className={cn(className)}>
        {lines.map((line, i) => (
          <CodeBlockLine key={i}>{line}</CodeBlockLine>
        ))}
      </CodeBlockContent>
    </CodeBlock>
  );
}
