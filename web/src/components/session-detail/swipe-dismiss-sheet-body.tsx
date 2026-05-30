"use client";

/**
 * Body of an iOS-style tools bottom sheet, designed to live inside a
 * `<SheetContent side="bottom">` from `@/components/ui/sheet`.
 *
 * Layered behaviour, top to bottom:
 *  1. Grabber row at the top — always drags the sheet.
 *  2. Sticky title row — also drags.
 *  3. Scroll region — only initiates a drag when scrollTop === 0 AND the
 *     gesture is going down. Otherwise the touch is consumed by the inner
 *     scroll, matching native iOS sheet semantics.
 *
 * Drag math:
 *  - We only translate by max(0, dy) so users can't drag the sheet upward.
 *  - On release, dismiss if dy > DISMISS_DISTANCE OR velocity > DISMISS_VELOCITY.
 *  - Otherwise spring back to 0 with a short transition.
 *
 * Animation strategy:
 *  - Initial open and natural close are owned by base-ui's CSS animations
 *    (the host `<SheetContent>`'s `data-starting-style` / `data-ending-style`).
 *  - We only mutate the popup's inline transform during a drag (or while
 *    restoring after a drag), then clear the inline styles so the next
 *    open re-animates with the library's defaults.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { Wrench } from "lucide-react";

import { Badge } from "@/components/ui/badge";

const SHEET_DISMISS_DISTANCE = 96;
const SHEET_DISMISS_VELOCITY = 0.55; // px per ms

export function SwipeDismissSheetBody({
  onDismiss,
  title,
  count,
  children,
  onScroll,
  onScrollRootChange,
}: {
  onDismiss: () => void;
  title: string;
  count: number;
  children: React.ReactNode;
  onScroll?: (e: React.UIEvent<HTMLDivElement>) => void;
  onScrollRootChange?: (node: HTMLDivElement | null) => void;
}) {
  const popupRef = useRef<HTMLDivElement | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<{
    startY: number;
    lastY: number;
    lastT: number;
    velocity: number;
    active: boolean;
    fromScroll: boolean;
  } | null>(null);
  const [dragY, setDragY] = useState(0);
  const [dragging, setDragging] = useState(false);
  // True while we're animating back to rest after a drag.
  const [restoring, setRestoring] = useState(false);

  useEffect(() => {
    const popup = popupRef.current?.parentElement;
    if (!popup) return;
    if (dragging) {
      popup.style.transition = "none";
      popup.style.transform = `translate3d(0, ${dragY}px, 0)`;
      return;
    }
    if (restoring) {
      popup.style.transition =
        "transform 220ms cubic-bezier(0.32, 0.72, 0, 1)";
      popup.style.transform = `translate3d(0, ${dragY}px, 0)`;
      const handle = window.setTimeout(() => {
        popup.style.transition = "";
        popup.style.transform = "";
        setRestoring(false);
      }, 240);
      return () => window.clearTimeout(handle);
    }
    // Idle: ensure no leftover inline styles fight base-ui's animations.
    popup.style.transition = "";
    popup.style.transform = "";
  }, [dragY, dragging, restoring]);

  const beginDrag = useCallback(
    (clientY: number, fromScroll: boolean) => {
      dragStateRef.current = {
        startY: clientY,
        lastY: clientY,
        lastT: performance.now(),
        velocity: 0,
        active: true,
        fromScroll,
      };
      setDragging(true);
    },
    [],
  );

  const updateDrag = useCallback((clientY: number) => {
    const s = dragStateRef.current;
    if (!s?.active) return;
    const now = performance.now();
    const dt = Math.max(1, now - s.lastT);
    s.velocity = (clientY - s.lastY) / dt;
    s.lastY = clientY;
    s.lastT = now;
    setDragY(Math.max(0, clientY - s.startY));
  }, []);

  const endDrag = useCallback(() => {
    const s = dragStateRef.current;
    if (!s?.active) return;
    s.active = false;
    const dy = Math.max(0, s.lastY - s.startY);
    const shouldDismiss =
      dy > SHEET_DISMISS_DISTANCE ||
      (dy > 24 && s.velocity > SHEET_DISMISS_VELOCITY);
    setDragging(false);
    if (shouldDismiss) {
      setDragY(0);
      onDismiss();
    } else {
      setRestoring(true);
      setDragY(0);
    }
  }, [onDismiss]);

  const handleHeaderTouchStart = useCallback(
    (e: React.TouchEvent) => {
      beginDrag(e.touches[0].clientY, false);
    },
    [beginDrag],
  );

  const handleScrollTouchStart = useCallback(
    (e: React.TouchEvent) => {
      const el = scrollRef.current;
      if (!el) return;
      if (el.scrollTop > 0) return;
      beginDrag(e.touches[0].clientY, true);
    },
    [beginDrag],
  );

  const setScrollRoot = useCallback(
    (node: HTMLDivElement | null) => {
      scrollRef.current = node;
      onScrollRootChange?.(node);
    },
    [onScrollRootChange],
  );

  const handleTouchMove = useCallback(
    (e: React.TouchEvent) => {
      const s = dragStateRef.current;
      if (!s?.active) return;
      const clientY = e.touches[0].clientY;
      const dy = clientY - s.startY;
      if (s.fromScroll && dy < 0) {
        s.active = false;
        setDragging(false);
        setDragY(0);
        return;
      }
      if (dy > 0 && e.cancelable) e.preventDefault();
      updateDrag(clientY);
    },
    [updateDrag],
  );

  return (
    <div ref={popupRef} className="flex min-h-0 h-full flex-col">
      <div
        onTouchStart={handleHeaderTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={endDrag}
        onTouchCancel={endDrag}
        className="flex cursor-grab justify-center pt-2.5 pb-1 active:cursor-grabbing"
      >
        <span
          className={[
            "block h-1 w-9 rounded-full transition-colors",
            dragging ? "bg-foreground/45" : "bg-foreground/20",
          ].join(" ")}
          aria-hidden
        />
      </div>

      <div
        onTouchStart={handleHeaderTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={endDrag}
        onTouchCancel={endDrag}
        className="flex items-center gap-2 border-b border-border/60 px-4 pb-3"
      >
        <Wrench className="size-4 text-muted-foreground" />
        <h2 className="font-display text-[15px] font-semibold text-foreground">
          {title}
        </h2>
        <Badge variant="secondary" className="ml-1 text-[10px]">
          {count}
        </Badge>
        <button
          type="button"
          onClick={onDismiss}
          className="ml-auto -mr-1 inline-flex h-9 items-center px-2 text-[14px] font-medium text-primary"
        >
          Done
        </button>
      </div>

      <div
        ref={setScrollRoot}
        onScroll={onScroll}
        onTouchStart={handleScrollTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={endDrag}
        onTouchCancel={endDrag}
        className={[
          "flex-1 space-y-2 overflow-y-auto px-3 pt-3",
          "pb-[calc(env(safe-area-inset-bottom)+0.75rem)]",
          "[-webkit-overflow-scrolling:touch] overscroll-contain",
        ].join(" ")}
      >
        {children}
      </div>
    </div>
  );
}
