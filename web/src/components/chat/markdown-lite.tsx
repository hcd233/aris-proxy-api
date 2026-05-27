"use client";

/**
 * MarkdownLite — a tiny, dependency-free Markdown renderer for chat content.
 *
 * Supports:
 *  - Fenced code blocks  ```lang\n...\n```
 *  - Inline code         `code`
 *  - Bold                **text**
 *  - Italic              *text*
 *  - Links               [text](url)
 *  - Headings            #, ##, ###
 *  - Unordered lists     - item / * item
 *  - Ordered lists       1. item
 *  - Blockquotes         > text
 *  - Horizontal rule     ---
 *
 * Anything not matched falls back to plain paragraphs with preserved newlines.
 * This is enough for LLM transcripts; if a more complete Markdown experience
 * is needed later, swap the implementation for `react-markdown`.
 */

import { useState } from "react";
import { Check, Copy } from "lucide-react";

import { cn } from "@/lib/utils";

// ─── Inline tokens ────────────────────────────────────────────────────────────

type InlineNode =
  | { kind: "text"; value: string }
  | { kind: "code"; value: string }
  | { kind: "bold"; children: InlineNode[] }
  | { kind: "italic"; children: InlineNode[] }
  | { kind: "link"; href: string; children: InlineNode[] };

// Order matters: code first (don't tokenize markers inside backticks), then
// links, then bold (`**`), then italic (`*` or `_`).
const INLINE_RE =
  /(`[^`\n]+`)|(\[[^\]\n]+\]\([^)\s]+\))|(\*\*[^*\n]+\*\*)|(\*[^*\n]+\*)|(_[^_\n]+_)/;

function parseInline(text: string): InlineNode[] {
  const nodes: InlineNode[] = [];
  let rest = text;

  while (rest.length > 0) {
    const m = rest.match(INLINE_RE);
    if (!m || m.index === undefined) {
      nodes.push({ kind: "text", value: rest });
      break;
    }
    if (m.index > 0) {
      nodes.push({ kind: "text", value: rest.slice(0, m.index) });
    }
    const token = m[0];
    if (token.startsWith("`")) {
      nodes.push({ kind: "code", value: token.slice(1, -1) });
    } else if (token.startsWith("[")) {
      const close = token.indexOf("](");
      const label = token.slice(1, close);
      const href = token.slice(close + 2, -1);
      nodes.push({ kind: "link", href, children: parseInline(label) });
    } else if (token.startsWith("**")) {
      nodes.push({ kind: "bold", children: parseInline(token.slice(2, -2)) });
    } else {
      // *italic* or _italic_
      nodes.push({ kind: "italic", children: parseInline(token.slice(1, -1)) });
    }
    rest = rest.slice(m.index + token.length);
  }
  return nodes;
}

function renderInline(nodes: InlineNode[], keyPrefix = ""): React.ReactNode {
  return nodes.map((n, i) => {
    const k = `${keyPrefix}${i}`;
    switch (n.kind) {
      case "text":
        return <span key={k}>{n.value}</span>;
      case "code":
        return (
          <code
            key={k}
            className="rounded-[5px] border border-border/60 bg-muted/70 px-1.5 py-[1px] font-mono text-[0.9em] text-foreground/90"
          >
            {n.value}
          </code>
        );
      case "bold":
        return (
          <strong key={k} className="font-semibold text-foreground">
            {renderInline(n.children, `${k}-`)}
          </strong>
        );
      case "italic":
        return (
          <em key={k} className="italic">
            {renderInline(n.children, `${k}-`)}
          </em>
        );
      case "link":
        return (
          <a
            key={k}
            href={n.href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary underline decoration-primary/30 underline-offset-2 transition-colors hover:decoration-primary"
          >
            {renderInline(n.children, `${k}-`)}
          </a>
        );
    }
  });
}

// ─── Block parser ─────────────────────────────────────────────────────────────

type Block =
  | { kind: "code"; lang: string; value: string }
  | { kind: "heading"; level: 1 | 2 | 3; text: string }
  | { kind: "list"; ordered: boolean; items: string[] }
  | { kind: "quote"; text: string }
  | { kind: "hr" }
  | { kind: "paragraph"; text: string };

function parseBlocks(src: string): Block[] {
  const lines = src.replace(/\r\n/g, "\n").split("\n");
  const blocks: Block[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Fenced code block
    const fence = line.match(/^```(\w+)?\s*$/);
    if (fence) {
      const lang = fence[1] ?? "";
      const buf: string[] = [];
      i += 1;
      while (i < lines.length && !/^```\s*$/.test(lines[i])) {
        buf.push(lines[i]);
        i += 1;
      }
      i += 1; // skip closing ``` (or EOF)
      blocks.push({ kind: "code", lang, value: buf.join("\n") });
      continue;
    }

    // Horizontal rule
    if (/^---+\s*$/.test(line)) {
      blocks.push({ kind: "hr" });
      i += 1;
      continue;
    }

    // Heading
    const heading = line.match(/^(#{1,3})\s+(.*)$/);
    if (heading) {
      const level = heading[1].length as 1 | 2 | 3;
      blocks.push({ kind: "heading", level, text: heading[2] });
      i += 1;
      continue;
    }

    // Blockquote (consecutive `> ` lines)
    if (/^>\s?/.test(line)) {
      const buf: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) {
        buf.push(lines[i].replace(/^>\s?/, ""));
        i += 1;
      }
      blocks.push({ kind: "quote", text: buf.join("\n") });
      continue;
    }

    // Unordered list
    if (/^[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^[-*]\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^[-*]\s+/, ""));
        i += 1;
      }
      blocks.push({ kind: "list", ordered: false, items });
      continue;
    }

    // Ordered list
    if (/^\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\d+\.\s+/, ""));
        i += 1;
      }
      blocks.push({ kind: "list", ordered: true, items });
      continue;
    }

    // Blank line — skip
    if (line.trim() === "") {
      i += 1;
      continue;
    }

    // Paragraph (consecutive non-blank, non-block lines)
    const buf: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !/^```/.test(lines[i]) &&
      !/^#{1,3}\s+/.test(lines[i]) &&
      !/^>\s?/.test(lines[i]) &&
      !/^[-*]\s+/.test(lines[i]) &&
      !/^\d+\.\s+/.test(lines[i]) &&
      !/^---+\s*$/.test(lines[i])
    ) {
      buf.push(lines[i]);
      i += 1;
    }
    if (buf.length > 0) {
      blocks.push({ kind: "paragraph", text: buf.join("\n") });
    }
  }

  return blocks;
}

// ─── Code block component ─────────────────────────────────────────────────────

function CodeBlock({ lang, value }: { lang: string; value: string }) {
  const [copied, setCopied] = useState(false);

  const onCopy = () => {
    void navigator.clipboard.writeText(value).then(() => {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    });
  };

  return (
    <div className="group/code my-3 overflow-hidden rounded-lg border border-border/60 bg-[#1F1A14] dark:bg-[#15110d]">
      <div className="flex items-center justify-between border-b border-white/5 px-3 py-1.5">
        <span className="font-mono text-[10px] uppercase tracking-[0.12em] text-white/40">
          {lang || "text"}
        </span>
        <button
          type="button"
          onClick={onCopy}
          className="flex items-center gap-1 rounded px-1.5 py-0.5 font-mono text-[10px] text-white/40 transition-colors hover:bg-white/5 hover:text-white/80"
          aria-label="Copy code"
        >
          {copied ? (
            <>
              <Check className="size-3" />
              copied
            </>
          ) : (
            <>
              <Copy className="size-3" />
              copy
            </>
          )}
        </button>
      </div>
      <pre className="overflow-x-auto px-4 py-3 font-mono text-[12.5px] leading-relaxed text-[#E8DDD3]">
        <code>{value}</code>
      </pre>
    </div>
  );
}

// ─── Public component ─────────────────────────────────────────────────────────

interface MarkdownLiteProps {
  text: string;
  /** When true, paragraphs preserve raw whitespace (used for system / refusal). */
  raw?: boolean;
  className?: string;
}

export function MarkdownLite({ text, raw = false, className }: MarkdownLiteProps) {
  if (!text) {
    return <span className="text-muted-foreground/60">—</span>;
  }
  if (raw) {
    return (
      <p className={cn("whitespace-pre-wrap break-words", className)}>{text}</p>
    );
  }

  const blocks = parseBlocks(text);

  return (
    <div className={cn("space-y-3 text-[15px] leading-[1.7]", className)}>
      {blocks.map((b, i) => {
        switch (b.kind) {
          case "code":
            return <CodeBlock key={i} lang={b.lang} value={b.value} />;
          case "hr":
            return <hr key={i} className="my-4 border-border/60" />;
          case "heading": {
            const cls =
              b.level === 1
                ? "font-display text-xl font-semibold tracking-tight"
                : b.level === 2
                  ? "font-display text-lg font-semibold tracking-tight"
                  : "font-display text-base font-semibold";
            return (
              <h3 key={i} className={cn("mt-4 mb-1 text-foreground", cls)}>
                {renderInline(parseInline(b.text))}
              </h3>
            );
          }
          case "quote":
            return (
              <blockquote
                key={i}
                className="border-l-2 border-primary/40 pl-4 text-muted-foreground italic"
              >
                {b.text.split("\n").map((ln, j) => (
                  <p key={j} className="whitespace-pre-wrap break-words">
                    {renderInline(parseInline(ln))}
                  </p>
                ))}
              </blockquote>
            );
          case "list":
            return b.ordered ? (
              <ol
                key={i}
                className="ml-5 list-decimal space-y-1 marker:text-muted-foreground/60"
              >
                {b.items.map((it, j) => (
                  <li key={j} className="break-words">
                    {renderInline(parseInline(it))}
                  </li>
                ))}
              </ol>
            ) : (
              <ul
                key={i}
                className="ml-5 list-disc space-y-1 marker:text-muted-foreground/60"
              >
                {b.items.map((it, j) => (
                  <li key={j} className="break-words">
                    {renderInline(parseInline(it))}
                  </li>
                ))}
              </ul>
            );
          case "paragraph":
            return (
              <p key={i} className="whitespace-pre-wrap break-words">
                {renderInline(parseInline(b.text))}
              </p>
            );
        }
      })}
    </div>
  );
}
