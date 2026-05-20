import axios from "axios"
import { API } from "./api-routes"
import { statsSchema, type Stats } from "./schemas/stats"
import { issueSchema, type Issue } from "./schemas/issue"
import { runSchema, type Run } from "./schemas/run"
import { repoSchema, type Repo } from "./schemas/repo"
import {
  listResponseSchema,
  detailResponseSchema,
  type PaginationMeta,
} from "./schemas/common"

const client = axios.create({ baseURL: "" })

export interface IssueFilters {
  repo?: string
  limit?: number
  offset?: number
}

export interface RunFilters {
  limit?: number
  offset?: number
}

export interface CreateIssueBody {
  repo: string
  title: string
  body?: string
  labels?: string[]
}

export interface DraftIssueBody {
  repo: string
  description: string
}

export interface DraftIssueResponse {
  title: string
  body: string
  labels: string[]
}

export interface DispatchBody {
  repo: string
  issue_number: number
}

export async function fetchStats(): Promise<Stats> {
  const { data } = await client.get(API.stats)
  const parsed = detailResponseSchema(statsSchema).parse(data)
  return parsed.data
}

export async function fetchRepos(): Promise<{
  data: Repo[]
  meta: PaginationMeta
}> {
  const { data } = await client.get(API.repos)
  return listResponseSchema(repoSchema).parse(data)
}

export async function fetchIssues(
  filters: IssueFilters = {},
): Promise<{ data: Issue[]; meta: PaginationMeta }> {
  const params = new URLSearchParams()
  if (filters.repo) params.set("repo", filters.repo)
  if (filters.limit) params.set("limit", String(filters.limit))
  if (filters.offset !== undefined) params.set("offset", String(filters.offset))

  const { data } = await client.get(`${API.issues}?${params}`)
  return listResponseSchema(issueSchema).parse(data)
}

export async function fetchIssue(
  repo: string,
  number: number,
): Promise<Issue> {
  const { data } = await client.get(API.issue(repo, number))
  const parsed = detailResponseSchema(issueSchema).parse(data)
  return parsed.data
}

export async function createIssue(
  body: CreateIssueBody,
): Promise<{ number: number; url: string }> {
  const { data } = await client.post(API.issues, body)
  return data.data
}

export async function draftIssue(
  body: DraftIssueBody,
): Promise<DraftIssueResponse> {
  const { data } = await client.post(API.issueDraft, body)
  return data.data
}

export async function fetchRuns(
  filters: RunFilters = {},
): Promise<{ data: Run[]; meta: PaginationMeta }> {
  const params = new URLSearchParams()
  if (filters.limit) params.set("limit", String(filters.limit))
  if (filters.offset !== undefined) params.set("offset", String(filters.offset))

  const { data } = await client.get(`${API.runs}?${params}`)
  return listResponseSchema(runSchema).parse(data)
}

export async function fetchRun(id: number): Promise<Run> {
  const { data } = await client.get(API.run(id))
  const parsed = detailResponseSchema(runSchema).parse(data)
  return parsed.data
}

export interface RunOutput {
  output: string
  live: boolean
}

export async function fetchRunOutput(id: number): Promise<RunOutput> {
  const { data } = await client.get(API.runOutput(id))
  return data.data
}

// --- Trace types and functions ---

export interface TraceEvent {
  phase: string
  at: string
  detail?: string
}

export async function fetchRunTrace(id: number): Promise<TraceEvent[]> {
  const { data } = await client.get(API.runTrace(id))
  return data.data
}

export async function dispatchAgent(
  body: DispatchBody,
): Promise<{ status: string }> {
  const { data } = await client.post(API.dispatch, body)
  return data.data
}

export async function triggerRebase(repo: string): Promise<{ status: string }> {
  const { data } = await client.post(API.rebase, { repo })
  return data.data
}

// --- PR types and functions ---

export interface PRSummary {
  number: number
  title: string
  repo: string
  author: string
  url: string
  review_decision: string
  created_at: string
}

export interface ReviewComment {
  id: number
  path: string
  line: number | null
  body: string
  author: string
  created_at: string
  in_reply_to_id: number | null
}

export interface PRFile {
  path: string
  additions: number
  deletions: number
}

export interface PRDetail {
  number: number
  title: string
  body: string
  head_ref: string
  base_ref: string
  url: string
  state: string
  additions: number
  deletions: number
  review_decision: string
  author: string
  created_at: string
  diff: string
  comments: ReviewComment[]
  files: PRFile[]
}

export interface PRReviewBody {
  event: "approve" | "request_changes" | "comment"
  body?: string
}

export async function fetchPRs(): Promise<{
  data: PRSummary[]
  meta: PaginationMeta
}> {
  const { data } = await client.get(API.prs)
  return data
}

export async function fetchPRDetail(
  repo: string,
  number: number,
): Promise<PRDetail> {
  const { data } = await client.get(API.prDetail(repo, number))
  return data.data
}

export async function submitPRReview(
  repo: string,
  number: number,
  body: PRReviewBody,
): Promise<{ status: string }> {
  const { data } = await client.post(API.prReview(repo, number), body)
  return data.data
}

export async function resolvePRComments(
  repo: string,
  number: number,
): Promise<{ status: string }> {
  const { data } = await client.post(API.prResolve(repo, number))
  return data.data
}

// --- Task types and functions ---

export interface CreateTaskBody {
  repo: string
  title: string
  prompt: string
}

export async function fetchTasks(
  filters: RunFilters = {},
): Promise<{ data: Run[]; meta: PaginationMeta }> {
  const params = new URLSearchParams()
  if (filters.limit) params.set("limit", String(filters.limit))
  if (filters.offset !== undefined) params.set("offset", String(filters.offset))

  const { data } = await client.get(`${API.tasks}?${params}`)
  return listResponseSchema(runSchema).parse(data)
}

export async function createTask(
  body: CreateTaskBody,
): Promise<{ status: string }> {
  const { data } = await client.post(API.tasks, body)
  return data.data
}

export async function dispatchAgentOnPR(
  repo: string,
  number: number,
  prompt: string,
): Promise<{ status: string }> {
  const { data } = await client.post(API.prAgent(repo, number), { prompt })
  return data.data
}
