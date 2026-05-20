"use client"

import { useRepos } from "@/hooks/use-repos"
import { PageHeading } from "@/components/layout/page-heading"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { ExternalLink } from "lucide-react"

export function ReposView() {
  const { data, isLoading } = useRepos()

  return (
    <>
      <PageHeading
        title="Repositories"
        description="Configured repos and poll status"
      />
      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-32" />
          ))}
        </div>
      ) : !data?.data.length ? (
        <p className="text-muted-foreground">No repos configured.</p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {data.data.map((repo) => (
            <Card key={repo.name}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base">{repo.name}</CardTitle>
                  <Badge variant={repo.enabled ? "default" : "secondary"}>
                    {repo.enabled ? "Enabled" : "Disabled"}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent className="space-y-2">
                {repo.remote && (
                  <a
                    href={`https://github.com/${repo.remote}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-blue-500 hover:underline inline-flex items-center gap-1"
                  >
                    {repo.remote}
                    <ExternalLink className="h-3 w-3" />
                  </a>
                )}
                <p className="text-xs text-muted-foreground font-mono truncate">
                  {repo.path}
                </p>
                {repo.last_poll_at && (
                  <p className="text-xs text-muted-foreground">
                    Last poll: {new Date(repo.last_poll_at).toLocaleString()}
                  </p>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </>
  )
}
