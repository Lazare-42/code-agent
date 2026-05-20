"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { LayoutDashboard, CircleDot, Play, Bot } from "lucide-react"
import { cn } from "@/lib/utils"

const navItems = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/issues", label: "Issues", icon: CircleDot },
  { href: "/runs", label: "Runs", icon: Play },
]

export function AppHeader() {
  const pathname = usePathname()

  return (
    <header className="flex md:hidden h-14 items-center gap-4 border-b bg-sidebar px-4">
      <Bot className="h-5 w-5 text-sidebar-primary" />
      <nav className="flex items-center gap-1">
        {navItems.map((item) => {
          const isActive = pathname.startsWith(item.href)
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm font-medium transition-colors",
                isActive
                  ? "bg-sidebar-accent text-sidebar-accent-foreground"
                  : "text-sidebar-foreground/70 hover:text-sidebar-accent-foreground"
              )}
            >
              <item.icon className="h-4 w-4" />
              <span className="hidden sm:inline">{item.label}</span>
            </Link>
          )
        })}
      </nav>
    </header>
  )
}
