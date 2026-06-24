"use client";

/**
 * Markdown — full GFM rendering for chat content.
 *
 * Engine: react-markdown + remark-gfm + remark-breaks + rehype-highlight + rehype-raw.
 * Component overrides preserve the existing visual style (warm code blocks
 * with copy button, primary-coloured links, muted blockquotes, etc.).
 *
 * Special block: ` ```mermaid ` fences are rendered with mermaid.js (lazy-loaded).
 *
 * The exported component is named `MarkdownLite` for backward compatibility
 * with existing call sites; despite the name it now supports the full GFM
 * surface (tables, task lists, strikethrough, autolinks) plus syntax-highlighted
 * code blocks and mermaid diagrams.
 */

import { useEffect, useRef, useState } from "react";
import { Check, Copy } from "lucide-react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkBreaks from "remark-breaks";
import rehypeHighlight from "rehype-highlight";
import rehypeRaw from "rehype-raw";

import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";

import "highlight.js/styles/atom-one-dark.css";

function isSafeUrl(url: unknown): url is string {
  return typeof url === "string" && (url.startsWith("http://") || url.startsWith("https://"));
}

function reactChildrenToText(children: ReactNode): string {
  if (typeof children === "string") return children;
  if (typeof children === "number") return String(children);
  if (Array.isArray(children)) return children.map(reactChildrenToText).join("");
  if (children && typeof children === "object" && "props" in (children as object)) {
    return reactChildrenToText((children as { props: { children?: ReactNode } }).props.children);
  }
  return "";
}

// ─── Mermaid block ───────────────────────────────────────────────────────────

let mermaidLoadPromise: Promise<typeof import("mermaid").default> | null = null;

function loadMermaid() {
  if (!mermaidLoadPromise) {
    mermaidLoadPromise = import("mermaid").then((m) => {
      m.default.initialize({
        startOnLoad: false,
        securityLevel: "strict",
        theme: "default",
        fontFamily: "var(--font-sans)",
      });
      return m.default;
    });
  }
  return mermaidLoadPromise;
}

let mermaidIDCounter = 0;

function MermaidBlock({ code }: { code: string }) {
  const ref = useRef<HTMLDivElement | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [svg, setSvg] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    mermaidIDCounter += 1;
    const id = `mermaid-${mermaidIDCounter}`;
    loadMermaid()
      .then((mermaid) => mermaid.render(id, code))
      .then((rsp) => {
        if (cancelled) return;
        setSvg(rsp.svg);
        setError(null);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [code]);

  if (error) {
    return (
      <div className="my-3 overflow-hidden rounded-lg border border-destructive/40 bg-destructive/5">
        <div className="border-b border-destructive/20 px-3 py-1.5 font-mono text-[10px] uppercase tracking-[0.12em] text-destructive">
          mermaid · render error
        </div>
        <pre className="overflow-x-auto px-4 py-3 font-mono text-[12px] leading-relaxed text-destructive">
          {error}
        </pre>
        <pre className="overflow-x-auto border-t border-destructive/20 bg-background/50 px-4 py-3 font-mono text-[12px] leading-relaxed text-muted-foreground">
          {code}
        </pre>
      </div>
    );
  }

  return (
    <div className="my-3 overflow-hidden rounded-lg border border-border/60 bg-card">
      <div className="border-b border-border/60 px-3 py-1.5 font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
        mermaid
      </div>
      <div
        ref={ref}
        className="flex justify-center overflow-x-auto px-4 py-4 [&>svg]:max-w-full [&>svg]:h-auto"
        dangerouslySetInnerHTML={svg ? { __html: svg } : undefined}
      />
    </div>
  );
}

// ─── Code block (with language label + copy button) ─────────────────────────

function CodeBlock({
  lang,
  value,
  children,
  highlightedClassName,
}: {
  lang: string;
  value: string;
  children?: ReactNode;
  highlightedClassName?: string;
}) {
  const [copied, setCopied] = useState(false);
  const t = useT();

  const onCopy = () => {
    void navigator.clipboard.writeText(value).then(() => {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    });
  };

  return (
    <div className="group/code my-3 overflow-hidden rounded-xl border border-[#3A322B] bg-[#26211C] dark:bg-[#1F1A14] dark:border-[#2A2520]">
      <div className="flex items-center justify-between border-b border-[#3A322B] px-3.5 py-1.5 dark:border-[#2A2520]">
        <span className="font-mono text-[10px] tracking-[0.12em] text-[#E8DDD3]/35">
          {lang || "text"}
        </span>
        <button
          type="button"
          onClick={onCopy}
          className="flex items-center gap-1 rounded px-1.5 py-0.5 font-mono text-[10px] text-[#E8DDD3]/35 transition-colors hover:bg-white/5 hover:text-[#E8DDD3]/70"
          aria-label={t("markdown.copy_code_aria")}
        >
          {copied ? (
            <>
              <Check className="size-3" />
              {t("markdown.copied")}
            </>
          ) : (
            <>
              <Copy className="size-3" />
              {t("markdown.copy")}
            </>
          )}
        </button>
      </div>
      <pre className="overflow-x-auto px-4 py-3 font-mono text-[12.5px] leading-[1.55] text-[#E8DDD3]">
        <code className={cn("hljs bg-transparent !p-0", highlightedClassName)}>
          {children ?? value}
        </code>
      </pre>
    </div>
  );
}

// ─── Component overrides for react-markdown ─────────────────────────────────

const markdownComponents: Components = {
  // Headings — use display serif font for h1/h2/h3
  h1: ({ children }) => (
    <h1 className="mt-5 mb-2 font-display text-2xl font-semibold tracking-tight text-foreground">
      {children}
    </h1>
  ),
  h2: ({ children }) => (
    <h2 className="mt-5 mb-2 font-display text-xl font-semibold tracking-tight text-foreground">
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3 className="mt-4 mb-1.5 font-display text-lg font-semibold tracking-tight text-foreground">
      {children}
    </h3>
  ),
  h4: ({ children }) => (
    <h4 className="mt-3 mb-1 font-display text-base font-semibold text-foreground">
      {children}
    </h4>
  ),

  // Paragraphs
  p: ({ children }) => (
    <p className="my-2 whitespace-pre-wrap break-words leading-[1.7]">
      {children}
    </p>
  ),

  // Inline elements
  strong: ({ children }) => (
    <strong className="font-semibold text-foreground">{children}</strong>
  ),
  em: ({ children }) => <em className="italic">{children}</em>,
  del: ({ children }) => (
    <del className="text-muted-foreground line-through decoration-muted-foreground/60">
      {children}
    </del>
  ),
  a: ({ href, children }) => {
    if (!isSafeUrl(href)) {
      return <span className="break-words">{children}</span>;
    }
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className="text-primary underline decoration-primary/30 underline-offset-2 transition-colors hover:decoration-primary"
      >
        {children}
      </a>
    );
  },

  // Lists
  ul: ({ children }) => (
    <ul className="my-2 ml-5 list-disc space-y-1 marker:text-muted-foreground/60">
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol className="my-2 ml-5 list-decimal space-y-1 marker:text-muted-foreground/60">
      {children}
    </ol>
  ),
  li: ({ children, ...props }) => {
    // GFM task list: react-markdown injects a checkbox <input type="checkbox" disabled>
    // as the first child. Detect it via className.
    const className = (props as { className?: string }).className ?? "";
    if (className.includes("task-list-item")) {
      return (
        <li className="-ml-5 flex list-none items-start gap-2 break-words">
          {children}
        </li>
      );
    }
    return <li className="break-words">{children}</li>;
  },
  input: ({ type, checked, disabled }) => {
    if (type === "checkbox") {
      return (
        <input
          type="checkbox"
          checked={checked}
          disabled={disabled}
          readOnly
          className="mt-[0.45em] size-3.5 shrink-0 rounded border-border accent-primary"
        />
      );
    }
    return null;
  },

  // Blockquote
  blockquote: ({ children }) => (
    <blockquote className="my-3 border-l-2 border-primary/40 pl-4 text-muted-foreground italic [&>p]:my-1">
      {children}
    </blockquote>
  ),

  // Horizontal rule
  hr: () => <hr className="my-4 border-border/60" />,

  // GFM table
  table: ({ children }) => (
    <div className="my-3 overflow-x-auto rounded-lg border border-border/60">
      <table className="w-full border-collapse text-[13px]">{children}</table>
    </div>
  ),
  thead: ({ children }) => (
    <thead className="bg-muted/50 text-foreground">{children}</thead>
  ),
  tbody: ({ children }) => <tbody>{children}</tbody>,
  tr: ({ children }) => (
    <tr className="border-b border-border/60 last:border-0">{children}</tr>
  ),
  th: ({ children, style }) => (
    <th
      style={style}
      className="px-3 py-2 text-left text-[12px] font-semibold uppercase tracking-wide text-foreground"
    >
      {children}
    </th>
  ),
  td: ({ children, style }) => (
    <td
      style={style}
      className="px-3 py-2 align-top text-foreground/90 [&>p]:my-0"
    >
      {children}
    </td>
  ),

  // Image
  img: ({ src, alt }) => {
    if (!isSafeUrl(src)) return null;
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={src}
        alt={alt ?? ""}
        className="my-2 inline-block max-h-80 max-w-full rounded-lg border border-border/60 bg-muted/40 object-contain"
      />
    );
  },

  // Code: inline + block + mermaid
  code: ({ className, children, ...props }) => {
    const inline = !(props as { node?: { tagName?: string } }).node;
    // react-markdown sets className like `language-xxx` only on block code.
    const langMatch = /language-([\w+-]+)/.exec(className ?? "");
    const lang = langMatch?.[1] ?? "";
    const value = reactChildrenToText(children).replace(/\n$/, "");

    // Inline code (no language class set by markdown parser)
    if (!lang && (inline || !value.includes("\n"))) {
      return (
        <code className="rounded-[5px] border border-border/60 bg-muted/70 px-1.5 py-[1px] font-mono text-[0.9em] text-foreground/90">
          {children}
        </code>
      );
    }

    if (lang === "mermaid") {
      return <MermaidBlock code={value} />;
    }

    return <CodeBlock lang={lang} value={value} highlightedClassName={className}>{children}</CodeBlock>;
  },

  // `pre` is rendered by our CodeBlock; suppress react-markdown's wrapper for
  // block code so we don't get a double <pre>.
  pre: ({ children }) => <>{children}</>,
};

// ─── Public component ────────────────────────────────────────────────────────

interface MarkdownProps {
  text: string;
  /** Render text verbatim (no markdown), used for system / refusal blocks. */
  raw?: boolean;
  className?: string;
}

export function MarkdownLite({ text, raw = false, className }: MarkdownProps) {
  if (!text) {
    return <span className="text-muted-foreground/60">—</span>;
  }
  if (raw) {
    return (
      <p className={cn("whitespace-pre-wrap break-words", className)}>{text}</p>
    );
  }

  return (
    <div className={cn("text-[15px] leading-[1.7]", className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkBreaks]}
        rehypePlugins={[rehypeRaw, [rehypeHighlight, { ignoreMissing: true, detect: true }]]}
        components={markdownComponents}
      >
        {text}
      </ReactMarkdown>
    </div>
  );
}
