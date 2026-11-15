import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import App from "./App";

afterEach(() => vi.restoreAllMocks());

describe("authentication", () => {
  it("uses the selected create-account action and opens the workspace", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const path = String(input);
      if (path.endsWith("/me")) return new Response("{}", { status: 401 });
      if (path.endsWith("/auth/register")) {
        return new Response(JSON.stringify({ id: "u1", email: "dev@example.test", createdAt: new Date().toISOString() }), { status: 201 });
      }
      if (path.endsWith("/projects")) return new Response("[]", { status: 200 });
      return new Response("{}", { status: 404 });
    });

    render(<App />);
    fireEvent.change(await screen.findByLabelText("Email address"), { target: { value: "dev@example.test" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "correct horse battery staple" } });
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));

    await screen.findByText("Your workspace");
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/v1/auth/register",
        expect.objectContaining({ method: "POST" }),
      );
    });
  });

  it("shows service failures instead of treating them as a signed-out session", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ error: { message: "database unavailable" } }), { status: 503 }),
    );

    render(<App />);

    expect(await screen.findByRole("alert")).toHaveTextContent("database unavailable");
    expect(screen.getByRole("heading", { name: "Keep good work moving." })).toBeInTheDocument();
  });
});
