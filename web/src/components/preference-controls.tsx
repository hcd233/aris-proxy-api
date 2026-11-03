"use client";

import { LanguageSwitcher } from "@/components/language-switcher";
import { ThemeSwitcher, type PreferenceTone } from "@/components/theme/theme-switcher";
import { cn } from "@/lib/utils";

/**
 * 全局偏好控件（语言 + 主题）的唯一实现：门户 / 登录 / 回调 / 分享 / 仪表盘侧栏共用。
 * horizontal 用于页头，vertical 用于折叠侧栏。
 */
export function PreferenceControls({
  tone = "page",
  orientation = "horizontal",
  compact,
  className,
  triggerClassName,
}: {
  tone?: PreferenceTone;
  orientation?: "horizontal" | "vertical";
  /** 语言只显示短码；默认竖排时开启 */
  compact?: boolean;
  className?: string;
  /** 追加到两个触发按钮上（例如把窄屏触控区放大到 44px） */
  triggerClassName?: string;
}) {
  const vertical = orientation === "vertical";
  return (
    <div className={cn("flex items-center gap-1.5", vertical && "flex-col", className)}>
      <LanguageSwitcher tone={tone} compact={compact ?? vertical} className={triggerClassName} />
      <ThemeSwitcher
        tone={tone}
        tooltipSide={vertical ? "right" : "bottom"}
        className={triggerClassName}
      />
    </div>
  );
}
