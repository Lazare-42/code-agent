"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { LayoutDashboard, CircleDot, Play, FolderGit2, Bot, GitPullRequest, Zap } from "lucide-react"
import { cn } from "@/lib/utils"

const navItems = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/issues", label: "Issues", icon: CircleDot },
  { href: "/runs", label: "Runs", icon: Play },
  { href: "/tasks", label: "Tasks", icon: Zap },
  { href: "/prs", label: "PRs", icon: GitPullRequest },
  { href: "/repos", label: "Repos", icon: FolderGit2 },
]

export function AppSidebar() {
  const pathname = usePathname()

  return (
    <aside className="hidden md:flex w-64 flex-col border-r bg-sidebar">
      <div className="flex h-14 items-center gap-2 border-b px-4">
        <Bot className="h-5 w-5 text-sidebar-primary" />
        <span className="font-semibold text-sidebar-foreground">Code Agent</span>
      </div>
      <nav className="flex-1 space-y-1 p-3">
        {navItems.map((item) => {
          const isActive = pathname.startsWith(item.href)
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-sidebar-accent text-sidebar-accent-foreground"
                  : "text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground"
              )}
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </Link>
          )
        })}
      </nav>
    </aside>
  )
}
