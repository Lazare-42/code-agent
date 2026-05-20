import type { Metadata } from "next"
import localFont from "next/font/local"
import "./globals.css"

import type { ReactNode } from "react"
import { Providers } from "@/components/providers"

// Geist bundled locally (OFL) instead of next/font/google for hermetic
// Nix builds; same CSS variables.
const geistSans = localFont({
  src: "./fonts/Geist.ttf",
  variable: "--font-geist-sans",
  weight: "100 900",
})

const geistMono = localFont({
  src: "./fonts/GeistMono.ttf",
  variable: "--font-geist-mono",
  weight: "100 900",
})

export const metadata: Metadata = {
  title: "Code Agent",
  description: "GitHub issue agent dispatcher dashboard",
}

export default function RootLayout({
  children,
}: Readonly<{
  children: ReactNode
}>) {
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable}`}
      suppressHydrationWarning
    >
      <body className="flex min-h-screen flex-col antialiased">
        <Providers>{children}</Providers>
      </body>
    </html>
  )
}
