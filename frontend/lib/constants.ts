export const RUN_STATUS_COLORS: Record<string, string> = {
  running:
    "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400",
  success:
    "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-400",
  failed:
    "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
  timeout:
    "bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400",
  rejected:
    "bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-400",
  skipped:
    "bg-zinc-100 text-zinc-800 dark:bg-zinc-900/30 dark:text-zinc-400",
}

export const VERDICT_COLORS: Record<string, string> = {
  approve:
    "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-400",
  reject:
    "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
}

export const REVIEW_DECISION_COLORS: Record<string, string> = {
  APPROVED:
    "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-400",
  CHANGES_REQUESTED:
    "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
  REVIEW_REQUIRED:
    "bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400",
}

export const ISSUE_STATE_COLORS: Record<string, string> = {
  open:
    "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
  closed:
    "bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400",
}
