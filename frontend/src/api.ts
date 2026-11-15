export type User = { id: string; email: string; createdAt: string };
export type Project = { id: string; name: string; createdAt: string };
export type Issue = {
  id: string;
  projectId: string;
  title: string;
  description: string;
  status: "open" | "in_progress" | "done";
  createdAt: string;
  updatedAt: string;
};
export type Comment = {
  id: string;
  issueId: string;
  authorId: string;
  body: string;
  createdAt: string;
};

export class APIError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = "APIError";
  }
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (init.method && init.method !== "GET") headers.set("X-Requested-With", "XMLHttpRequest");
  const response = await fetch(`/api/v1${path}`, {
    ...init,
    headers,
    credentials: "same-origin",
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as
      | { error?: { message?: string } }
      | null;
    throw new APIError(body?.error?.message ?? `Request failed (${response.status})`, response.status);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}
