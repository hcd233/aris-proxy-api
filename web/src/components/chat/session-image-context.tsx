"use client";

import { createContext, useContext, type ReactNode } from "react";

const SessionImageContext = createContext<number | null>(null);

/** 只有会话详情页提供 sessionId；分享页等未提供时图片卡片不可点击。 */
export function SessionImageProvider({
  sessionId,
  children,
}: {
  sessionId: number;
  children: ReactNode;
}) {
  return <SessionImageContext.Provider value={sessionId}>{children}</SessionImageContext.Provider>;
}

export function useSessionImageId(): number | null {
  return useContext(SessionImageContext);
}
