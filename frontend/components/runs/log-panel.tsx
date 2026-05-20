"use client"

import { useEffect, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { fetchRunOutput, fetchRunTrace, type TraceEvent } from "@/lib/api-client"

interface RunLogPanelProps {
  runId: number
  live: boolean
}

export function RunLogPanel({ runId, live }: RunLogPanelProps) {
  const [tab, setTab] = useState<"log" | "trace">("log")
  const bottomRef = useRef<HTMLDivElement>(null)

  const { data, isLoading } = useQuery({
    queryKey: ["run-output", runId],
    queryFn: () => fetchRunOutput(runId),
    refetchInterval: live ? 3000 : false,
  })

  const { data: traceData } = useQuery({
    queryKey: ["run-trace", runId],
    queryFn: () => fetchRunTrace(runId),
    enabled: tab === "trace",
  })

  useEffect(() => {
    if (tab === "log") {
      bottomRef.current?.scrollIntoView({ behavior: "smooth" })
    }
  }, [data?.output, tab])

  return (
    <div>
      <div className="mb-2 flex gap-2 text-xs">
        <button
          onClick={() => setTab("log")}
          className={`rounded px-2 py-1 ${tab === "log" ? "bg-zinc-700 text-white" : "text-zinc-400 hover:text-zinc-200"}`}
        >
          Log
        </button>
        <button
          onClick={() => setTab("trace")}
          className={`rounded px-2 py-1 ${tab === "trace" ? "bg-zinc-700 text-white" : "text-zinc-400 hover:text-zinc-200"}`}
        >
          Trace
        </button>
      </div>

      {tab === "log" ? (
        <div className="relative max-h-96 overflow-auto rounded bg-zinc-950 p-3 font-mono text-xs text-zinc-300">
          {live && (
            <div className="sticky top-0 right-0 float-right text-green-400 text-xs animate-pulse">
              live
            </div>
          )}
          {isLoading ? (
            <span className="text-muted-foreground">Loading logs...</span>
          ) : data?.output ? (
            <pre className="whitespace-pre-wrap break-words">{data.output}</pre>
          ) : (
            <span className="text-muted-foreground">No output available.</span>
          )}
          <div ref={bottomRef} />
        </div>
      ) : (
        <TraceTimeline events={traceData ?? []} />
      )}
    </div>
  )
}

function TraceTimeline({ events }: { events: TraceEvent[] }) {
  if (events.length === 0) {
    return (
      <div className="rounded bg-zinc-950 p-3 text-xs text-muted-foreground">
        No trace data available.
      </div>
    )
  }

  return (
    <div className="rounded bg-zinc-950 p-3 text-xs text-zinc-300">
      <div className="space-y-1">
        {events.map((event, i) => {
          const at = new Date(event.at)
          const prev = i > 0 ? new Date(events[i - 1].at) : null
          const delta = prev ? ((at.getTime() - prev.getTime()) / 1000).toFixed(1) : null

          return (
            <div key={i} className="flex items-baseline gap-3 font-mono">
              <span className="w-20 shrink-0 text-zinc-500">
                {at.toLocaleTimeString()}
              </span>
              {delta !== null && (
                <span className="w-16 shrink-0 text-right text-zinc-600">
                  +{delta}s
                </span>
              )}
              {delta === null && <span className="w-16 shrink-0" />}
              <span className="font-semibold text-blue-400">{event.phase}</span>
              {event.detail && (
                <span className="truncate text-zinc-500">{event.detail}</span>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
