"use client"

import { useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { usePRDetail } from "@/hooks/use-prs"
import {
  submitPRReview,
  resolvePRComments,
  dispatchAgentOnPR,
} from "@/lib/api-client"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"

interface PRDetailPanelProps {
  repo: string
  number: number
}

export function PRDetailPanel({ repo, number }: PRDetailPanelProps) {
  const { data, isLoading } = usePRDetail(repo, number)
  const queryClient = useQueryClient()
  const [showDiff, setShowDiff] = useState(false)
  const [agentPrompt, setAgentPrompt] = useState("")

  const reviewMutation = useMutation({
    mutationFn: (event: "approve" | "request_changes") =>
      submitPRReview(repo, number, { event }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["pr-detail", repo, number] })
      queryClient.invalidateQueries({ queryKey: ["prs"] })
    },
  })

  const resolveMutation = useMutation({
    mutationFn: () => resolvePRComments(repo, number),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["prs"] })
    },
  })

  const agentMutation = useMutation({
    mutationFn: (prompt: string) => dispatchAgentOnPR(repo, number, prompt),
    onSuccess: () => {
      setAgentPrompt("")
      queryClient.invalidateQueries({ queryKey: ["prs"] })
    },
  })

  if (isLoading) {
    return (
      <div className="space-y-2 p-4">
        <Skeleton className="h-4 w-3/4" />
        <Skeleton className="h-4 w-1/2" />
        <Skeleton className="h-20 w-full" />
      </div>
    )
  }

  if (!data) return null

  // Group comments by file
  const commentsByFile = new Map<string, typeof data.comments>()
  for (const c of data.comments) {
    const existing = commentsByFile.get(c.path) ?? []
    existing.push(c)
    commentsByFile.set(c.path, existing)
  }

  return (
    <div className="space-y-4 p-4">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="font-mono text-muted-foreground">
              {data.head_ref} &larr; {data.base_ref}
            </span>
          </div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span className="text-green-500">+{data.additions}</span>
            <span className="text-red-500">-{data.deletions}</span>
            <span>&middot;</span>
            <span>{data.files.length} files</span>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => reviewMutation.mutate("approve")}
            disabled={reviewMutation.isPending}
          >
            Approve
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => reviewMutation.mutate("request_changes")}
            disabled={reviewMutation.isPending}
          >
            Request Changes
          </Button>
          {data.comments.length > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => resolveMutation.mutate()}
              disabled={resolveMutation.isPending}
            >
              {resolveMutation.isPending
                ? "Dispatching..."
                : "Resolve Comments"}
            </Button>
          )}
        </div>
      </div>

      {reviewMutation.isSuccess && (
        <p className="text-sm text-emerald-500">Review submitted.</p>
      )}
      {resolveMutation.isSuccess && (
        <p className="text-sm text-blue-500">
          Agent dispatched to resolve comments.
        </p>
      )}

      {/* Run Agent */}
      <div className="rounded border p-3 space-y-2">
        <h4 className="text-sm font-medium">Run Agent</h4>
        <textarea
          className="w-full rounded bg-zinc-950 border border-zinc-800 p-2 text-sm text-zinc-300 placeholder:text-zinc-600 focus:outline-none focus:ring-1 focus:ring-zinc-600 min-h-[80px]"
          rows={3}
          placeholder="Describe what the agent should do on this PR branch..."
          value={agentPrompt}
          onChange={(e) => setAgentPrompt(e.target.value)}
        />
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            onClick={() => agentMutation.mutate(agentPrompt)}
            disabled={agentMutation.isPending || !agentPrompt.trim()}
          >
            {agentMutation.isPending ? "Dispatching..." : "Dispatch Agent"}
          </Button>
          {agentMutation.isSuccess && (
            <span className="text-sm text-blue-500">
              Agent dispatched — check Runs page for progress.
            </span>
          )}
        </div>
      </div>

      {/* PR body */}
      {data.body && (
        <div className="rounded bg-zinc-950 p-3 text-sm text-zinc-300">
          <pre className="whitespace-pre-wrap break-words">{data.body}</pre>
        </div>
      )}

      {/* Changed files list */}
      <div>
        <h4 className="text-sm font-medium mb-2">
          Changed Files ({data.files.length})
        </h4>
        <div className="space-y-1">
          {data.files.map((f) => (
            <div
              key={f.path}
              className="flex items-center gap-2 text-sm font-mono"
            >
              <span className="text-green-500">+{f.additions}</span>
              <span className="text-red-500">-{f.deletions}</span>
              <span className="text-muted-foreground">{f.path}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Review comments */}
      {data.comments.length > 0 && (
        <div>
          <h4 className="text-sm font-medium mb-2">
            Review Comments ({data.comments.length})
          </h4>
          <div className="space-y-3">
            {Array.from(commentsByFile.entries()).map(([path, comments]) => (
              <div key={path} className="rounded border p-3 space-y-2">
                <div className="font-mono text-sm text-muted-foreground">
                  {path}
                </div>
                {comments.map((c) => (
                  <div
                    key={c.id}
                    className={`rounded bg-zinc-950 p-2 text-sm ${
                      c.in_reply_to_id
                        ? "ml-4 border-l-2 border-zinc-700"
                        : ""
                    }`}
                  >
                    <div className="flex items-center gap-2 text-xs text-muted-foreground mb-1">
                      <span className="font-medium text-zinc-300">
                        {c.author}
                      </span>
                      {c.line && (
                        <Badge variant="outline" className="text-xs">
                          L{c.line}
                        </Badge>
                      )}
                      <span>
                        {new Date(c.created_at).toLocaleDateString()}
                      </span>
                    </div>
                    <pre className="whitespace-pre-wrap break-words text-zinc-300">
                      {c.body}
                    </pre>
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Diff toggle */}
      <div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setShowDiff(!showDiff)}
        >
          {showDiff ? "Hide Diff" : "Show Diff"}
        </Button>
        {showDiff && data.diff && (
          <div className="mt-2 max-h-96 overflow-auto rounded bg-zinc-950 p-3 font-mono text-xs text-zinc-300">
            <pre className="whitespace-pre-wrap break-words">{data.diff}</pre>
          </div>
        )}
      </div>
    </div>
  )
}
