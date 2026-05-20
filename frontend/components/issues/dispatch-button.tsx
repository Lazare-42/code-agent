"use client"

import { useState } from "react"
import { useMutation } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Play } from "lucide-react"
import { toast } from "sonner"
import { dispatchAgent } from "@/lib/api-client"

interface DispatchButtonProps {
  repo: string
  issueNumber: number
}

export function DispatchButton({ repo, issueNumber }: DispatchButtonProps) {
  const [confirmed, setConfirmed] = useState(false)

  const mutation = useMutation({
    mutationFn: () => dispatchAgent({ repo, issue_number: issueNumber }),
    onSuccess: () => {
      toast.success(`Agent dispatched for ${repo} #${issueNumber}`)
      setConfirmed(false)
    },
    onError: (err: Error) => {
      toast.error(`Dispatch failed: ${err.message}`)
      setConfirmed(false)
    },
  })

  if (confirmed) {
    return (
      <div className="flex items-center gap-1">
        <Button
          size="sm"
          variant="destructive"
          onClick={() => mutation.mutate()}
          disabled={mutation.isPending}
          className="h-7 text-xs"
        >
          {mutation.isPending ? "..." : "Confirm"}
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => setConfirmed(false)}
          disabled={mutation.isPending}
          className="h-7 text-xs"
        >
          Cancel
        </Button>
      </div>
    )
  }

  return (
    <Button
      size="sm"
      variant="outline"
      onClick={() => setConfirmed(true)}
      className="h-7 text-xs gap-1"
    >
      <Play className="h-3 w-3" />
      Dispatch
    </Button>
  )
}
