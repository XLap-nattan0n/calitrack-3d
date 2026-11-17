import type { Metadata } from "next";
import "./globals.css";
import { messages } from "@/constants/messages";

export const metadata: Metadata = {
  title: messages.app.name,
  description: messages.app.tagline,
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="min-h-screen font-sans">{children}</body>
    </html>
  );
}
