"use client"

import { createColumnHelper } from "@tanstack/react-table"
import { Badge } from "@/components/ui/badge"
import { REVIEW_DECISION_COLORS } from "@/lib/constants"
import type { PRSummary } from "@/lib/api-client"

const columnHelper = createColumnHelper<PRSummary>()

export const prColumns = [
  columnHelper.accessor("repo", {
    header: "Repo",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("number", {
    header: "#",
    cell: (info) => (
      <span className="font-mono">#{info.getValue()}</span>
    ),
    size: 70,
  }),
  columnHelper.accessor("title", {
    header: "Title",
    cell: (info) => (
      <span className="truncate max-w-xs block">{info.getValue()}</span>
    ),
  }),
  columnHelper.accessor("author", {
    header: "Author",
    cell: (info) => (
      <span className="text-muted-foreground">{info.getValue()}</span>
    ),
    size: 120,
  }),
  columnHelper.accessor("review_decision", {
    header: "Review",
    cell: (info) => {
      const decision = info.getValue()
      if (!decision)
        return <span className="text-muted-foreground">-</span>
      const label = decision.replace(/_/g, " ").toLowerCase()
      return (
        <Badge
          variant="secondary"
          className={REVIEW_DECISION_COLORS[decision] ?? ""}
        >
          {label}
        </Badge>
      )
    },
    size: 140,
  }),
  columnHelper.accessor("created_at", {
    header: "Created",
    cell: (info) => {
      const d = new Date(info.getValue())
      return (
        <span className="text-muted-foreground text-sm">
          {d.toLocaleDateString()}
        </span>
      )
    },
    size: 100,
  }),
]
