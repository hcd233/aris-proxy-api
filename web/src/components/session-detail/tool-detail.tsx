"use client";

import { Braces, ChevronRight, FileText } from "lucide-react";
import type { UnifiedTool } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { HighlightedCode } from "@/components/chat/highlighted-code";
import { useT } from "@/lib/i18n";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";

const SECTION_LABEL =
  "mb-1.5 flex items-center gap-1 text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground";

/** 工具抽屉详情页：完整描述 → 参数 → 默认收起的原始 schema */
export function ToolDetail({ tool }: { tool: UnifiedTool }) {
  const t = useT();
  const params = tool.parameters;
  const paramProperties = (params?.properties as Record<string, Record<string, unknown>>) ?? {};
  const requiredParams = (params?.required as string[]) ?? [];
  const paramEntries = Object.entries(paramProperties);
  const hasSchema = !!params && Object.keys(params).length > 0;

  return (
    <div className="flex flex-col gap-5">
      <section>
        <p className={SECTION_LABEL}>
          <FileText className="size-3" />
          {t("tool_sidebar.description")}
        </p>
        <p className="text-[13px] leading-relaxed whitespace-pre-wrap break-words text-foreground/85">
          {tool.description?.trim() || t("tool_sidebar.no_description")}
        </p>
      </section>

      <section>
        <p className={SECTION_LABEL}>
          <Braces className="size-3" />
          {t("tool_sidebar.parameters")}
        </p>
        {paramEntries.length === 0 ? (
          <p className="text-[12px] text-muted-foreground">{t("tool_sidebar.no_parameters")}</p>
        ) : (
          <ul className="divide-y divide-border/50">
            {paramEntries.map(([name, schema]) => (
              <li key={name} className="py-2 first:pt-0 last:pb-0">
                <div className="flex items-start gap-1.5">
                  <span className="min-w-0 font-mono text-[12.5px] font-medium break-all text-foreground">
                    {name}
                  </span>
                  {requiredParams.includes(name) && (
                    <span className="shrink-0 pt-px text-[11px] font-medium text-rose-500">
                      {t("tool_sidebar.required")}
                    </span>
                  )}
                  {schema.type !== undefined && (
                    <Badge
                      variant="secondary"
                      className="ml-auto shrink-0 px-1.5 py-0 font-mono text-[11px]"
                    >
                      {String(schema.type)}
                    </Badge>
                  )}
                </div>
                {typeof schema.description === "string" && schema.description.trim() && (
                  <p className="mt-1 text-[12px] leading-relaxed whitespace-pre-wrap break-words text-muted-foreground">
                    {schema.description.trim()}
                  </p>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      {hasSchema && (
        <Collapsible>
          <CollapsibleTrigger className="flex items-center gap-1 text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:text-foreground">
            <ChevronRight className="size-3 transition-transform in-data-panel-open:rotate-90" />
            {t("tool_sidebar.raw_schema")}
          </CollapsibleTrigger>
          <CollapsibleContent>
            <HighlightedCode
              code={JSON.stringify(params, null, 2)}
              language="json"
              size="sm"
              containerClassName="mt-2"
              className="max-h-80"
            />
          </CollapsibleContent>
        </Collapsible>
      )}
    </div>
  );
}
