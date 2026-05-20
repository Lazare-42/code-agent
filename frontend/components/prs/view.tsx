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
import { PageHeading } from "@/components/layout/page-heading"
import { usePRs } from "@/hooks/use-prs"
import { prColumns } from "./columns"
import { PRDetailPanel } from "./pr-detail"
import type { PRSummary } from "@/lib/api-client"

export function PRsView() {
  const { data, isLoading } = usePRs()
  const [expandedKey, setExpandedKey] = useState<string | null>(null)

  const prs = data?.data ?? []

  const table = useReactTable({
    data: prs,
    columns: prColumns,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <>
      <PageHeading title="Pull Requests" description="Open PRs across all repos" />
      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : (
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
                  <TableHead>Details</TableHead>
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.length ? (
                table.getRowModel().rows.map((row) => {
                  const pr = row.original
                  const key = `${pr.repo}/${pr.number}`
                  const isExpanded = expandedKey === key
                  return (
                    <>
                      <TableRow
                        key={row.id}
                        className="cursor-pointer"
                        onClick={() =>
                          setExpandedKey(isExpanded ? null : key)
                        }
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
                              setExpandedKey(isExpanded ? null : key)
                            }}
                          >
                            {isExpanded ? "Hide" : "View"}
                          </Button>
                        </TableCell>
                      </TableRow>
                      {isExpanded && (
                        <TableRow key={`${row.id}-detail`}>
                          <TableCell
                            colSpan={prColumns.length + 1}
                            className="p-0"
                          >
                            <PRDetailPanel
                              repo={pr.repo}
                              number={pr.number}
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
                    colSpan={prColumns.length + 1}
                    className="h-24 text-center"
                  >
                    No open pull requests found.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      )}
    </>
  )
}
