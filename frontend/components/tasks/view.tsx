"use client"

import { useState } from "react"
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
} from "@tanstack/react-table"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { useTasks } from "@/hooks/use-tasks"
import { useRepos } from "@/hooks/use-repos"
import { createTask, type RunFilters } from "@/lib/api-client"
import { PageHeading } from "@/components/layout/page-heading"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Select } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { RunLogPanel } from "@/components/runs/log-panel"
import { taskColumns } from "./columns"
import type { Run } from "@/lib/schemas/run"
import type { PaginationMeta } from "@/lib/schemas/common"

export function TasksView() {
  const [filters, setFilters] = useState<RunFilters>({ limit: 20 })
  const { data, isLoading } = useTasks(filters)

  return (
    <>
      <PageHeading title="Tasks" description="Standalone agent tasks — no issue required" />
      <div className="space-y-6">
        <CreateTaskForm />
        <TasksTable
          data={data?.data ?? []}
          meta={data?.meta ?? { total: 0, limit: 20, offset: 0 }}
          isLoading={isLoading}
          onPageChange={(offset) => setFilters((f) => ({ ...f, offset }))}
        />
      </div>
    </>
  )
}

function CreateTaskForm() {
  const [open, setOpen] = useState(false)
  const [repo, setRepo] = useState("")
  const [title, setTitle] = useState("")
  const [prompt, setPrompt] = useState("")

  const { data: repos } = useRepos()
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: createTask,
    onSuccess: () => {
      toast.success("Task dispatched")
      queryClient.invalidateQueries({ queryKey: ["tasks"] })
      queryClient.invalidateQueries({ queryKey: ["runs"] })
      setOpen(false)
      setRepo("")
      setTitle("")
      setPrompt("")
    },
    onError: (err: Error) => {
      toast.error(`Failed to dispatch task: ${err.message}`)
    },
  })

  if (!open) {
    return (
      <Button onClick={() => setOpen(true)} size="sm">
        New Task
      </Button>
    )
  }

  return (
    <div className="rounded-lg border p-4 space-y-3 bg-card">
      <h3 className="font-semibold">New Task</h3>
      <Select value={repo} onChange={(e) => setRepo(e.target.value)}>
        <option value="">Select repo...</option>
        {repos?.data.map((r) => (
          <option key={r.name} value={r.name}>
            {r.name}
          </option>
        ))}
      </Select>
      <Input
        placeholder="Task title"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <Textarea
        placeholder="Prompt for the agent..."
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
        rows={5}
      />
      <div className="flex gap-2">
        <Button
          size="sm"
          disabled={!repo || !title || !prompt || mutation.isPending}
          onClick={() => mutation.mutate({ repo, title, prompt })}
        >
          {mutation.isPending ? "Dispatching..." : "Dispatch"}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setOpen(false)}
        >
          Cancel
        </Button>
      </div>
    </div>
  )
}

interface TasksTableProps {
  data: Run[]
  meta: PaginationMeta
  isLoading: boolean
  onPageChange: (offset: number) => void
}

function TasksTable({ data, meta, isLoading, onPageChange }: TasksTableProps) {
  const [expandedId, setExpandedId] = useState<number | null>(null)

  const table = useReactTable({
    data,
    columns: taskColumns,
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
                        <TableCell colSpan={taskColumns.length + 1} className="p-2">
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
                  colSpan={taskColumns.length + 1}
                  className="h-24 text-center"
                >
                  No tasks found.
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
