import { afterEach, describe, expect, it, vi } from "vitest";
import { request } from "./api";

afterEach(() => vi.restoreAllMocks());

describe("request", () => {
  it("adds the JSON and CSRF headers to writes", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ id: "p1" }), { status: 201 }),
    );

    await request<{ id: string }>("/projects", { method: "POST", body: '{"name":"x"}' });

    const [, init] = fetchMock.mock.calls[0];
    const headers = new Headers(init?.headers);
    expect(headers.get("Content-Type")).toBe("application/json");
    expect(headers.get("X-Requested-With")).toBe("XMLHttpRequest");
    expect(init?.credentials).toBe("same-origin");
  });

  it("surfaces API errors instead of hiding them", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ error: { message: "project not found" } }), { status: 404 }),
    );

    await expect(request("/projects/missing/issues")).rejects.toThrow("project not found");
  });
});
