"use client";

import { useEffect, useId, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";

/** 只多出这么几行时不折叠：为藏一两行而多一次点击不划算 */
const FOLD_GRACE_LINES = 2;

/** 用户 / 助手消息折叠后保留的行数 */
export const MESSAGE_FOLD_LINES = 12;
/** 系统提示通常很长且信息密度低，保留更少 */
export const SYSTEM_FOLD_LINES = 6;

interface FoldedContentProps {
  /** 折叠时保留的可见行数（按容器自身行高计，渲染后的视觉行，不是源文本换行数） */
  lines: number;
  children: ReactNode;
  className?: string;
}

/**
 * 长消息折叠：渲染完整内容，高度超过 lines + FOLD_GRACE_LINES 行时默认只露出前 lines 行，
 * 底部渐隐并给出展开 / 收起按钮。按渲染高度而非字符数判断，markdown 标题、代码块、
 * 图片都算在内；ResizeObserver 跟踪图片、mermaid 等异步撑高。
 */
export function FoldedContent({ lines, children, className }: FoldedContentProps) {
  const t = useT();
  const id = useId();
  const clipRef = useRef<HTMLDivElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const [long, setLong] = useState(false);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const clip = clipRef.current;
    const body = bodyRef.current;
    if (!clip || !body) return;
    const observer = new ResizeObserver(() => {
      const style = getComputedStyle(clip);
      const lineHeight = parseFloat(style.lineHeight) || parseFloat(style.fontSize) * 1.5;
      setLong(body.offsetHeight > (lines + FOLD_GRACE_LINES) * lineHeight);
    });
    observer.observe(body);
    return () => observer.disconnect();
  }, [lines]);

  const folded = long && !open;

  const toggle = () => {
    setOpen((v) => !v);
    // 收起后内容变矮，按钮可能已滚出视口上方；把消息拉回可见区
    if (open) requestAnimationFrame(() => clipRef.current?.scrollIntoView({ block: "nearest" }));
  };

  return (
    <div className={cn("flex min-w-0 flex-col", className)}>
      <div
        ref={clipRef}
        id={id}
        data-folded={folded || undefined}
        style={{ "--fold-lines": lines } as CSSProperties}
        className="min-w-0 data-folded:max-h-[calc(var(--fold-lines)*1lh)] data-folded:overflow-hidden data-folded:[mask-image:linear-gradient(to_bottom,#000_calc(100%-2.5lh),transparent)]"
      >
        <div ref={bodyRef} className="flow-root">
          {children}
        </div>
      </div>
      {long && (
        <Button
          variant="link"
          size="xs"
          aria-expanded={open}
          aria-controls={id}
          onClick={(e) => {
            e.stopPropagation();
            toggle();
          }}
          className="mt-1 h-auto w-fit min-w-0 px-0"
        >
          {open ? t("collapsible.show_less") : t("collapsible.show_more")}
          <ChevronDown
            data-icon="inline-end"
            className={cn("transition-transform", open && "rotate-180")}
          />
        </Button>
      )}
    </div>
  );
}
