"use client"

import { createColumnHelper } from "@tanstack/react-table"
import { Badge } from "@/components/ui/badge"
import { RUN_STATUS_COLORS } from "@/lib/constants"
import type { Run } from "@/lib/schemas/run"

const columnHelper = createColumnHelper<Run>()

export const taskColumns = [
  columnHelper.accessor("id", {
    header: "ID",
    cell: (info) => (
      <span className="font-mono text-muted-foreground">{info.getValue()}</span>
    ),
    size: 50,
  }),
  columnHelper.accessor("repo", {
    header: "Repo",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("task_title", {
    header: "Title",
    cell: (info) => (
      <span className="font-medium">{info.getValue() ?? "-"}</span>
    ),
  }),
  columnHelper.accessor("status", {
    header: "Status",
    cell: (info) => {
      const status = info.getValue()
      return (
        <Badge
          variant="secondary"
          className={RUN_STATUS_COLORS[status] ?? ""}
        >
          {status}
        </Badge>
      )
    },
    size: 100,
  }),
  columnHelper.accessor("pr_url", {
    header: "PR",
    cell: (info) => {
      const url = info.getValue()
      if (!url) return <span className="text-muted-foreground">-</span>
      return (
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          className="text-blue-500 hover:underline text-sm"
        >
          View PR
        </a>
      )
    },
    size: 80,
  }),
  columnHelper.accessor("duration_s", {
    header: "Duration",
    cell: (info) => {
      const d = info.getValue()
      if (d == null) return <span className="text-muted-foreground">-</span>
      const mins = Math.floor(d / 60)
      const secs = Math.round(d % 60)
      return (
        <span className="text-muted-foreground text-sm">
          {mins > 0 ? `${mins}m ${secs}s` : `${secs}s`}
        </span>
      )
    },
    size: 90,
  }),
]
