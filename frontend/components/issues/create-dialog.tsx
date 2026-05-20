"use client"

import { useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Select } from "@/components/ui/select"
import { useRepos } from "@/hooks/use-repos"
import { createIssue, draftIssue } from "@/lib/api-client"

type Phase = "input" | "preview"

export function CreateIssueDialog() {
  const [open, setOpen] = useState(false)
  const [phase, setPhase] = useState<Phase>("input")

  // Phase 1 state
  const [repo, setRepo] = useState("")
  const [description, setDescription] = useState("")

  // Phase 2 state (editable draft)
  const [title, setTitle] = useState("")
  const [body, setBody] = useState("")
  const [labels, setLabels] = useState("")

  const { data: repos } = useRepos()
  const queryClient = useQueryClient()

  const draftMutation = useMutation({
    mutationFn: draftIssue,
    onSuccess: (data) => {
      setTitle(data.title)
      setBody(data.body)
      setLabels(data.labels?.join(", ") ?? "")
      setPhase("preview")
    },
    onError: (err: Error) => {
      toast.error(`Failed to generate draft: ${err.message}`)
    },
  })

  const createMutation = useMutation({
    mutationFn: createIssue,
    onSuccess: (data) => {
      toast.success(`Issue #${data.number} created`)
      queryClient.invalidateQueries({ queryKey: ["issues"] })
      queryClient.invalidateQueries({ queryKey: ["stats"] })
      resetAndClose()
    },
    onError: (err: Error) => {
      toast.error(`Failed to create issue: ${err.message}`)
    },
  })

  function resetAndClose() {
    setOpen(false)
    setPhase("input")
    setRepo("")
    setDescription("")
    setTitle("")
    setBody("")
    setLabels("")
  }

  if (!open) {
    return (
      <Button onClick={() => setOpen(true)} size="sm">
        Create Issue
      </Button>
    )
  }

  return (
    <div className="rounded-lg border p-4 space-y-3 bg-card">
      <h3 className="font-semibold">Create Issue</h3>

      {phase === "input" ? (
        <>
          <Select
            value={repo}
            onChange={(e) => setRepo(e.target.value)}
          >
            <option value="">Select repo...</option>
            {repos?.data.map((r) => (
              <option key={r.name} value={r.name}>
                {r.name}
              </option>
            ))}
          </Select>
          <Textarea
            placeholder="Describe the issue you want to create..."
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={5}
          />
          <div className="flex gap-2">
            <Button
              size="sm"
              disabled={!repo || !description || draftMutation.isPending}
              onClick={() =>
                draftMutation.mutate({ repo, description })
              }
            >
              {draftMutation.isPending ? "Generating..." : "Generate Draft"}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={resetAndClose}
            >
              Cancel
            </Button>
          </div>
        </>
      ) : (
        <>
          <Input
            placeholder="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <Textarea
            placeholder="Body (markdown)"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={8}
          />
          <Input
            placeholder="Labels (comma-separated)"
            value={labels}
            onChange={(e) => setLabels(e.target.value)}
          />
          <div className="flex gap-2">
            <Button
              size="sm"
              disabled={!title || createMutation.isPending}
              onClick={() =>
                createMutation.mutate({
                  repo,
                  title,
                  body: body || undefined,
                  labels: labels
                    ? labels.split(",").map((l) => l.trim()).filter(Boolean)
                    : undefined,
                })
              }
            >
              {createMutation.isPending ? "Creating..." : "Create Issue"}
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={draftMutation.isPending}
              onClick={() =>
                draftMutation.mutate({ repo, description })
              }
            >
              {draftMutation.isPending ? "Regenerating..." : "Regenerate"}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPhase("input")}
            >
              Back
            </Button>
          </div>
        </>
      )}
    </div>
  )
}
