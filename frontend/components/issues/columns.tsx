"use client"

import { createColumnHelper } from "@tanstack/react-table"
import { Badge } from "@/components/ui/badge"
import { ExternalLink } from "lucide-react"
import { ISSUE_STATE_COLORS } from "@/lib/constants"
import type { Issue } from "@/lib/schemas/issue"
import { DispatchButton } from "./dispatch-button"

const columnHelper = createColumnHelper<Issue>()

export const issueColumns = [
  columnHelper.accessor("number", {
    header: "#",
    cell: (info) => {
      const url = info.row.original.url
      const num = info.getValue()
      if (url) {
        return (
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-blue-500 hover:underline inline-flex items-center gap-1"
          >
            #{num}
            <ExternalLink className="h-3 w-3" />
          </a>
        )
      }
      return (
        <span className="font-mono text-muted-foreground">#{num}</span>
      )
    },
    size: 80,
  }),
  columnHelper.accessor("title", {
    header: "Title",
    cell: (info) => (
      <span className="font-medium truncate max-w-md block">
        {info.getValue()}
      </span>
    ),
  }),
  columnHelper.accessor("repo", {
    header: "Repo",
    cell: (info) => {
      const repoUrl = info.row.original.repo_url
      const name = info.getValue()
      if (repoUrl) {
        return (
          <a
            href={repoUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="text-blue-500 hover:underline"
          >
            {name}
          </a>
        )
      }
      return <span className="text-muted-foreground">{name}</span>
    },
  }),
  columnHelper.accessor("labels", {
    header: "Labels",
    cell: (info) => {
      const labels = info.getValue()
      if (!labels?.length) return null
      return (
        <div className="flex gap-1 flex-wrap">
          {labels.map((label) => (
            <Badge key={label} variant="outline" className="text-xs">
              {label}
            </Badge>
          ))}
        </div>
      )
    },
  }),
  columnHelper.accessor("state", {
    header: "State",
    cell: (info) => {
      const state = info.getValue()
      return (
        <Badge
          variant="secondary"
          className={ISSUE_STATE_COLORS[state] ?? ""}
        >
          {state}
        </Badge>
      )
    },
    size: 80,
  }),
  columnHelper.display({
    id: "actions",
    header: "",
    cell: (info) => (
      <DispatchButton
        repo={info.row.original.repo}
        issueNumber={info.row.original.number}
      />
    ),
    size: 100,
  }),
]
