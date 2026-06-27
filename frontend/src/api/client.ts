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
