"use client"

import { useState } from "react"
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
} from "@tanstack/react-table"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import { Button } from "@/components/ui/button"
import { runColumns } from "./columns"
import { RunLogPanel } from "./log-panel"
import type { Run } from "@/lib/schemas/run"
import type { PaginationMeta } from "@/lib/schemas/common"

interface RunsTableProps {
  data: Run[]
  meta: PaginationMeta
  isLoading: boolean
  onPageChange: (offset: number) => void
}

export function RunsTable({
  data,
  meta,
  isLoading,
  onPageChange,
}: RunsTableProps) {
  const [expandedId, setExpandedId] = useState<number | null>(null)

  const table = useReactTable({
    data,
    columns: runColumns,
    getCoreRowModel: getCoreRowModel(),
  })

  if (isLoading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                          header.column.columnDef.header,
                          header.getContext(),
                        )}
                  </TableHead>
                ))}
                <TableHead>Logs</TableHead>
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => {
                const run = row.original
                const isExpanded = expandedId === run.id
                return (
                  <>
                    <TableRow
                      key={row.id}
                      className="cursor-pointer"
                      onClick={() => setExpandedId(isExpanded ? null : run.id)}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id}>
                          {flexRender(
                            cell.column.columnDef.cell,
                            cell.getContext(),
                          )}
                        </TableCell>
                      ))}
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={(e) => {
                            e.stopPropagation()
                            setExpandedId(isExpanded ? null : run.id)
                          }}
                        >
                          {isExpanded ? "Hide" : "View"}
                        </Button>
                      </TableCell>
                    </TableRow>
                    {isExpanded && (
                      <TableRow key={`${row.id}-log`}>
                        <TableCell colSpan={runColumns.length + 1} className="p-2">
                          <RunLogPanel
                            runId={run.id}
                            live={run.status === "running"}
                          />
                        </TableCell>
                      </TableRow>
                    )}
                  </>
                )
              })
            ) : (
              <TableRow>
                <TableCell
                  colSpan={runColumns.length + 1}
                  className="h-24 text-center"
                >
                  No runs found.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      {meta.total > meta.limit && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Showing {meta.offset + 1}-
            {Math.min(meta.offset + meta.limit, meta.total)} of {meta.total}
          </p>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => onPageChange(Math.max(0, meta.offset - meta.limit))}
              disabled={meta.offset === 0}
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => onPageChange(meta.offset + meta.limit)}
              disabled={meta.offset + meta.limit >= meta.total}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
