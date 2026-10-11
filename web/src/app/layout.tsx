import type { Metadata } from "next";
// Noto Sans SC Variable — the single UI face. One unicode-range-split family
// covers Latin + Han + kana across a 100–900 weight axis, so no locale falls
// back to a system font and no weight needs its own file.
import "@fontsource-variable/noto-sans-sc";
// Maple Mono CN (monospace, CJK + Latin, unicode-range subset, loaded on
// demand) — reserved for code, IDs, keys and figures. Only the two weights
// reachable through the "Maple Mono CN" family name are imported: the
// package registers Medium/SemiBold under their own family names, which no
// font-family in this app references, so importing them only ships ~18MB of
// fonts that can never be selected.
import "@chinese-fonts/maple-mono-cn/dist/MapleMono-CN-Regular/result.css";
import "@chinese-fonts/maple-mono-cn/dist/MapleMono-CN-Bold/result.css";
import { AuthProvider } from "@/lib/auth-context";
import { I18nProvider } from "@/lib/i18n";
import { THEME_INIT_SCRIPT, ThemeProvider } from "@/lib/theme";
import { HtmlLangUpdater } from "@/components/html-lang-updater";
import { Toaster } from "@/components/ui/sonner";
import { ParticleBackground } from "@/components/theme/particle-background";
import { ThemeSwitcher } from "@/components/theme/theme-switcher";
import "./globals.css";

export const metadata: Metadata = {
  title: "Aris Proxy API",
  description: "Management interface for Aris Proxy API",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full antialiased" suppressHydrationWarning>
      <body className="min-h-full flex flex-col">
        {/* Raw inline script: runs synchronously during HTML parsing as
            the first element of <body>, before any visible content is
            parsed/painted, so [data-theme] is correct on first paint
            and there is no light→dark flash on full-page loads (login
            OAuth flow). Do NOT switch to next/script — its
            beforeInteractive strategy is deferred via the __next_s
            queue and only runs after the React bundle loads. */}
        <script dangerouslySetInnerHTML={{ __html: THEME_INIT_SCRIPT }} />
        <I18nProvider>
          <ThemeProvider>
            <HtmlLangUpdater />
            <AuthProvider>{children}</AuthProvider>
            <ParticleBackground />
            <ThemeSwitcher />
            <Toaster />
          </ThemeProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
