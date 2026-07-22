import type { Metadata } from "next";
import { Geist, Source_Serif_4, JetBrains_Mono } from "next/font/google";
import { AuthProvider } from "@/lib/auth-context";
import { I18nProvider } from "@/lib/i18n";
import { HtmlLangUpdater } from "@/components/html-lang-updater";
import { Toaster } from "@/components/ui/sonner";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist",
  subsets: ["latin"],
});

const sourceSerif = Source_Serif_4({
  variable: "--font-source-serif",
  subsets: ["latin"],
});

const jetbrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
});

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
    <html
      lang="en"
      className={`h-full antialiased ${geistSans.variable} ${sourceSerif.variable} ${jetbrainsMono.variable}`}
      suppressHydrationWarning
    >
      <body className="min-h-full flex flex-col">
        <I18nProvider>
          <HtmlLangUpdater />
          <AuthProvider>{children}</AuthProvider>
          <Toaster />
        </I18nProvider>
      </body>
    </html>
  );
}