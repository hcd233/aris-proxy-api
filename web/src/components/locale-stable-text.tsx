import { Children, type ReactNode } from "react";
import { localeVariants } from "@/lib/locale-variants";

/**
 * Renders a translated label sized to its widest translation: all variants
 * share one grid cell and only the current one is visible, so the element
 * keeps its width across locales. Non-translation text renders as-is.
 */
export function LocaleStableText({ text }: { text: string }) {
  const variants = localeVariants(text);
  if (!variants) return text;
  return (
    <span data-slot="locale-stable" className="inline-grid justify-items-center">
      {variants.map((v) => (
        <span
          key={v}
          className={v === text ? "col-start-1 row-start-1" : "invisible col-start-1 row-start-1"}
          aria-hidden={v === text ? undefined : true}
        >
          {v}
        </span>
      ))}
    </span>
  );
}

/** Wraps the direct string children of a rigid element (button label etc.). */
export function stabilizeLabels(children: ReactNode): ReactNode {
  return Children.map(children, (child) =>
    typeof child === "string" ? <LocaleStableText text={child} /> : child,
  );
}
