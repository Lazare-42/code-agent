"use client"

import { AppSidebar } from "@/components/layout/app-sidebar"
import { AppHeader } from "@/components/layout/app-header"

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-1 h-full">
      <AppSidebar />
      <div className="flex flex-1 flex-col min-h-0">
        <AppHeader />
        <main className="flex-1 overflow-y-auto p-6">{children}</main>
      </div>
    </div>
  )
}
