export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public body?: unknown
  ) {
    super(message);
  }
}

function csrfToken(): string | null {
  // Flask-WTF reads header X-CSRFToken; the token is in a csrf_cookie OR inlined by the SPA bootstrap.
  const m = document.cookie.match(/(?:^|;)\s*csrf_access_token=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : ((window as any).__CSRF__ ?? null);
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const csrf = csrfToken();
  if (csrf && init.method && init.method !== "GET") {
    headers.set("X-CSRFToken", csrf);
  }
  headers.set("Accept", "application/json");
  headers.set("X-Requested-With", "XMLHttpRequest");

  const res = await fetch(path, { ...init, headers, credentials: "same-origin" });
  if (!res.ok) {
    let body: unknown;
    try {
      body = await res.json();
    } catch {
      try {
        body = await res.text();
      } catch {
        body = null;
      }
    }
    throw new ApiError(res.status, `API ${res.status} ${path}`, body);
  }
  if (res.status === 204) return undefined as T;
  const ct = res.headers.get("Content-Type") ?? "";
  return (ct.includes("application/json") ? res.json() : res.text()) as Promise<T>;
}

import {
  DatasetSummary,
  DatasetDetail,
  CreateDatasetPayload,
  AddBooksPayload,
  ExportPayload,
  ExportChunkedPayload,
} from "../types/dataset";
import { BookListResponse } from "../types/book";

export interface BookListQ {
  offset: number;
  limit: number;
  search?: string;
  sort?: string;
  order?: string;
}

function buildQs(params: any): string {
  const s = new URLSearchParams();
  for (const k in params) {
    if (params[k] !== undefined && params[k] !== null && params[k] !== "") {
      s.set(k, String(params[k]));
    }
  }
  const str = s.toString();
  return str ? "?" + str : "";
}

export interface TaskStatus {
  task_id: string;
  user: string;
  taskMessage: string;
  progress: string;
  stat: number;
  is_cancellable: boolean;
  error: string;
  status: "Started" | "Waiting" | "Finished" | "Failed" | "Cancelled" | "Ended";
}

export const datasetsApi = {
  list:       (search?: string) => api<DatasetSummary[]>(`/api/v1/datasets${buildQs({ search })}`),
  get:        (id: number) => api<DatasetDetail>(`/api/v1/dataset/${id}`),
  create:     (p: CreateDatasetPayload) => api<DatasetDetail>(`/api/v1/datasets`, { method: "POST", body: JSON.stringify(p) }),
  update:     (id: number, p: Partial<CreateDatasetPayload>) => api<DatasetDetail>(`/api/v1/dataset/${id}`, { method: "PATCH", body: JSON.stringify(p) }),
  remove:     (id: number) => api<void>(`/api/v1/dataset/${id}`, { method: "DELETE" }),
  books:      (id: number, q: BookListQ) => api<BookListResponse>(`/api/v1/dataset/${id}/books${buildQs(q)}`),
  available:  (id: number, q: BookListQ) => api<BookListResponse>(`/api/v1/dataset/${id}/available-books${buildQs(q)}`),
  addBooks:   (id: number, p: AddBooksPayload) => api<{ added: number; skipped: number }>(`/api/v1/dataset/${id}/books`, { method: "POST", body: JSON.stringify(p) }),
  removeBook: (id: number, bookId: number) => api<void>(`/api/v1/dataset/${id}/books/${bookId}`, { method: "DELETE" }),
  export:        (id: number, p: ExportPayload) => api<{ task_id: string }>(`/api/v1/dataset/${id}/export`, { method: "POST", body: JSON.stringify(p) }),
  exportChunked: (id: number, p: ExportChunkedPayload) => api<{ task_id: string }>(`/api/v1/dataset/${id}/export-chunked`, { method: "POST", body: JSON.stringify(p) }),
  tasks:         () => api<TaskStatus[]>("/ajax/emailstat"),
  cancelTask:    (taskId: string) => api<{ success: boolean }>("/ajax/canceltask", { method: "POST", body: JSON.stringify({ task_id: taskId }) }),
  stats:         () => api<StatsData>("/api/v1/stats"),
};

export interface FormatStat {
  format: string;
  count: number;
  size: number;
}

export interface StatsData {
  version: string;
  total_books: number;
  total_authors: number;
  total_series: number;
  total_tags: number;
  total_publishers: number;
  read_books: number;
  unread_books: number;
  archived_books: number;
  formats: FormatStat[];
}
