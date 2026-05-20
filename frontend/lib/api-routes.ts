export const API = {
  stats: "/api/stats",
  repos: "/api/repos",
  issues: "/api/issues",
  issueDraft: "/api/issues/draft",
  issue: (repo: string, number: number) => `/api/issues/${repo}/${number}`,
  runs: "/api/runs",
  run: (id: number) => `/api/runs/${id}`,
  runOutput: (id: number) => `/api/runs/${id}/output`,
  dispatch: "/api/dispatch",
  rebase: "/api/rebase",
  prs: "/api/prs",
  prDetail: (repo: string, number: number) => `/api/prs/${repo}/${number}`,
  prReview: (repo: string, number: number) =>
    `/api/prs/${repo}/${number}/review`,
  prResolve: (repo: string, number: number) =>
    `/api/prs/${repo}/${number}/resolve`,
  prAgent: (repo: string, number: number) =>
    `/api/prs/${repo}/${number}/agent`,
  tasks: "/api/tasks",
  runTrace: (id: number) => `/api/runs/${id}/trace`,
} as const
