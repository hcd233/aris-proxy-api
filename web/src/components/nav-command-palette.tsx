"use client";

import { useCallback, useMemo, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { Autocomplete } from "@base-ui/react/autocomplete";
import { Search, type LucideIcon } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useI18n } from "@/lib/i18n";
import { NAV_GROUPS, matchesNavQuery, resolveNavGroups } from "@/lib/nav";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import {
  TooltipContent,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface PaletteItem {
  value: string;
  label: string;
  href: string;
  group: string;
  icon: LucideIcon;
}

interface PaletteGroup {
  value: string;
  items: PaletteItem[];
}

const subscribeNothing = () => () => {};
const detectMac = () => /mac|iphone|ipad/i.test(navigator.userAgent);

/** 快捷键提示用 ⌘ 还是 Ctrl；服务端渲染按非 Mac，水合后修正（useSyncExternalStore 无闪烁）。 */
export function useShortcutLabel(): string {
  const isMac = useSyncExternalStore(subscribeNothing, detectMac, () => false);
  return isMac ? "⌘K" : "Ctrl K";
}

/**
 * ⌘K / Ctrl+K 页面跳转面板：Dialog + Autocomplete inline（Base UI 官方 command palette 形态）。
 * 数据源与侧栏同一份 NAV_GROUPS，经 resolveNavGroups 按身份裁剪；demo 未开放的模块不可进入，不列出。
 * 面板内容在关闭时卸载，所以输入与高亮项每次打开都是干净的。
 */
export function NavCommandPalette({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t, locale } = useI18n();
  const router = useRouter();
  const { isAdmin, isDemo, isModuleOpen } = useAuth();
  const admin = isAdmin();
  const demo = isDemo();

  const groups = useMemo<PaletteGroup[]>(
    () =>
      resolveNavGroups(NAV_GROUPS, { isAdmin: admin, isDemo: demo, isModuleOpen }).flatMap(
        (group) => {
          const items = group.items
            .filter((item) => !item.locked)
            .map((item) => ({
              value: item.href,
              label: t(item.labelKey),
              href: item.href,
              group: t(group.labelKey),
              icon: item.icon,
            }));
          return items.length > 0 ? [{ value: t(group.labelKey), items }] : [];
        },
      ),
    // locale 驱动翻译文本刷新（t 引用稳定，见 lib/i18n.tsx）
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [admin, demo, isModuleOpen, locale, t],
  );

  const go = useCallback(
    (href: string) => {
      onOpenChange(false);
      router.push(href);
    },
    [onOpenChange, router],
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="top-[16%] max-h-[min(30rem,calc(100dvh-6rem))] -translate-y-0 grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden p-0 sm:max-w-lg"
      >
        <DialogTitle className="sr-only">{t("nav.search.trigger")}</DialogTitle>
        <Autocomplete.Root
          open
          inline
          items={groups}
          autoHighlight="always"
          keepHighlight
          filter={(item: PaletteItem, query: string) =>
            matchesNavQuery(query, [item.label, item.href, item.group])
          }
          itemToStringValue={(item: PaletteItem) => item.label}
        >
          <div className="flex items-center gap-2 border-b border-border px-3">
            <Search className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <Autocomplete.Input
              aria-label={t("nav.search.trigger")}
              placeholder={t("nav.search.placeholder")}
              className="h-12 w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-muted-foreground any-pointer-coarse:text-base"
            />
          </div>
          <div className="min-h-0 overflow-y-auto overscroll-contain py-1.5">
            <Autocomplete.Empty>
              <p className="px-4 py-8 text-center text-sm text-muted-foreground">
                {t("nav.search.empty")}
              </p>
            </Autocomplete.Empty>
            <Autocomplete.List>
              {(group: PaletteGroup) => (
                <Autocomplete.Group key={group.value} items={group.items} className="px-1.5">
                  <Autocomplete.GroupLabel className="px-2.5 pt-2 pb-1 text-xs text-muted-foreground select-none">
                    {group.value}
                  </Autocomplete.GroupLabel>
                  <Autocomplete.Collection>
                    {(item: PaletteItem) => {
                      const Icon = item.icon;
                      return (
                        <Autocomplete.Item
                          key={item.value}
                          value={item}
                          onClick={() => go(item.href)}
                          className="flex min-h-10 cursor-default items-center gap-3 rounded-lg px-2.5 text-sm outline-none select-none data-highlighted:bg-accent data-highlighted:text-accent-foreground"
                        >
                          <Icon
                            className="size-4 shrink-0 text-muted-foreground"
                            aria-hidden="true"
                          />
                          <span className="min-w-0 flex-1">{item.label}</span>
                        </Autocomplete.Item>
                      );
                    }}
                  </Autocomplete.Collection>
                </Autocomplete.Group>
              )}
            </Autocomplete.List>
          </div>
        </Autocomplete.Root>
      </DialogContent>
    </Dialog>
  );
}

/** 侧栏顶部的搜索入口：展开态是带快捷键提示的整行按钮，折叠态收成图标按钮。 */
export function NavSearchButton({
  collapsed,
  onClick,
}: {
  collapsed: boolean;
  onClick: () => void;
}) {
  const { t } = useI18n();
  const shortcut = useShortcutLabel();
  const label = t("nav.search.trigger");

  if (collapsed) {
    return (
      <div className="flex justify-center px-2 pb-3">
        <TooltipProvider>
          <TooltipRoot>
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={onClick}
                  aria-label={label}
                  className="text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-foreground"
                />
              }
            >
              <Search className="size-4" />
            </TooltipTrigger>
            <TooltipContent side="right">
              {label} · {shortcut}
            </TooltipContent>
          </TooltipRoot>
        </TooltipProvider>
      </div>
    );
  }

  return (
    <div className="px-2 pb-3">
      <Button
        variant="outline"
        onClick={onClick}
        className="h-8 w-full justify-start gap-2.5 rounded-md border-sidebar-border bg-sidebar-accent/40 px-2.5 font-normal text-sidebar-foreground/70 shadow-none hover:bg-sidebar-accent hover:text-sidebar-foreground"
      >
        <Search className="size-4 shrink-0" />
        <span className="min-w-0 flex-1 text-left">{label}</span>
        <kbd className="shrink-0 rounded border border-sidebar-border px-1.5 font-mono text-[11px] leading-5 text-sidebar-foreground/75">
          {shortcut}
        </kbd>
      </Button>
    </div>
  );
}
