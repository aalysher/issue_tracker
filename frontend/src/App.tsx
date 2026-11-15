import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import { APIError, request } from "./api";
import type { Comment, Issue, Project, User } from "./api";

const statuses: Issue["status"][] = ["open", "in_progress", "done"];

function App() {
  const [user, setUser] = useState<User | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState("");
  const [issues, setIssues] = useState<Issue[]>([]);
  const [openComments, setOpenComments] = useState<string | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const loadProjects = useCallback(async () => {
    const result = await request<Project[]>("/projects");
    setProjects(result);
    setSelectedProject((current) =>
      result.some((project) => project.id === current) ? current : (result[0]?.id ?? ""),
    );
  }, []);

  const loadIssues = useCallback(async (projectId: string) => {
    if (!projectId) {
      setIssues([]);
      return;
    }
    setIssues(await request<Issue[]>(`/projects/${projectId}/issues`));
  }, []);

  useEffect(() => {
    request<User>("/me")
      .then(setUser)
      .catch((reason: unknown) => {
        if (!(reason instanceof APIError && reason.status === 401)) showError(reason);
      });
  }, []);

  useEffect(() => {
    if (user) void loadProjects().catch(showError);
  }, [loadProjects, user]);

  useEffect(() => {
    if (user) void loadIssues(selectedProject).catch(showError);
  }, [loadIssues, selectedProject, user]);

  function showError(reason: unknown) {
    setError(reason instanceof Error ? reason.message : "Something went wrong");
  }

  async function submitAuth(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setBusy(true);
    const submitter = (event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null;
    const form = new FormData(event.currentTarget);
    const mode = submitter?.value === "register" ? "register" : "login";
    try {
      const currentUser = await request<User>(`/auth/${mode}`, {
        method: "POST",
        body: JSON.stringify({
          email: form.get("email"),
          password: form.get("password"),
        }),
      });
      setUser(currentUser);
    } catch (reason) {
      showError(reason);
    } finally {
      setBusy(false);
    }
  }

  async function signOut() {
    try {
      await request<void>("/auth/logout", { method: "POST" });
      setUser(null);
      setProjects([]);
      setIssues([]);
    } catch (reason) {
      showError(reason);
    }
  }

  async function createProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(event.currentTarget);
    try {
      const project = await request<Project>("/projects", {
        method: "POST",
        body: JSON.stringify({ name: form.get("name") }),
      });
      await loadProjects();
      setSelectedProject(project.id);
      formElement.reset();
    } catch (reason) {
      showError(reason);
    }
  }

  async function createIssue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedProject) return;
    const formElement = event.currentTarget;
    const form = new FormData(event.currentTarget);
    try {
      await request<Issue>(`/projects/${selectedProject}/issues`, {
        method: "POST",
        body: JSON.stringify({
          title: form.get("title"),
          description: form.get("description"),
        }),
      });
      await loadIssues(selectedProject);
      formElement.reset();
    } catch (reason) {
      showError(reason);
    }
  }

  async function changeStatus(issue: Issue, status: Issue["status"]) {
    try {
      await request<Issue>(`/issues/${issue.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status }),
      });
      await loadIssues(selectedProject);
    } catch (reason) {
      showError(reason);
    }
  }

  async function toggleComments(issueID: string) {
    if (openComments === issueID) {
      setOpenComments(null);
      return;
    }
    try {
      setOpenComments(issueID);
      setComments(await request<Comment[]>(`/issues/${issueID}/comments`));
    } catch (reason) {
      showError(reason);
    }
  }

  async function createComment(event: FormEvent<HTMLFormElement>, issueID: string) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(event.currentTarget);
    try {
      await request<Comment>(`/issues/${issueID}/comments`, {
        method: "POST",
        body: JSON.stringify({ body: form.get("body") }),
      });
      setComments(await request<Comment[]>(`/issues/${issueID}/comments`));
      formElement.reset();
    } catch (reason) {
      showError(reason);
    }
  }

  if (!user) {
    return (
      <main className="auth-page">
        <section className="auth-card">
          <div className="brand-mark">F</div>
          <p className="eyebrow">A calmer way to ship</p>
          <h1>Keep good work moving.</h1>
          <p className="muted">A small issue tracker built as a real-world agent workflow lab.</p>
          {error && <p className="notice" role="alert">{error}</p>}
          <form onSubmit={submitAuth} className="stack">
            <label>Email address<input name="email" type="email" required autoComplete="email" /></label>
            <label>Password<input name="password" type="password" minLength={12} required autoComplete="current-password" /></label>
            <div className="auth-actions">
              <button name="mode" value="login" disabled={busy}>Sign in</button>
              <button className="secondary" name="mode" value="register" disabled={busy}>Create account</button>
            </div>
          </form>
          <p className="hint">New accounts require at least 12 characters in a password.</p>
        </section>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <a className="wordmark" href="/">fieldnotes<span>.</span></a>
        <p className="side-label">WORKSPACE</p>
        <label className="project-select-label" htmlFor="project-select">Project</label>
        <select id="project-select" value={selectedProject} onChange={(event) => setSelectedProject(event.target.value)}>
          <option value="">Choose a project</option>
          {projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
        </select>
        <form className="new-project" onSubmit={createProject}>
          <label htmlFor="project-name">Create a project</label>
          <div className="inline-input">
            <input id="project-name" name="name" placeholder="Project name" maxLength={120} required />
            <button aria-label="Add project">+</button>
          </div>
        </form>
        <div className="sidebar-bottom">
          <div className="avatar">{user.email.slice(0, 1).toUpperCase()}</div>
          <div className="account">
            <strong>{user.email}</strong>
            <button className="link-button" onClick={signOut}>Sign out</button>
          </div>
        </div>
      </aside>

      <section className="main-content">
        <header className="topbar"><span>Workspace / {projects.find((item) => item.id === selectedProject)?.name ?? "Overview"}</span><span className="online-dot">All changes saved</span></header>
        <div className="content-wrap">
          <p className="eyebrow">PROJECT OVERVIEW</p>
          <div className="page-heading">
            <div><h1>{projects.find((item) => item.id === selectedProject)?.name ?? "Your workspace"}</h1><p className="muted">Make progress visible. Keep the next step clear.</p></div>
            <span className="issue-count">{issues.length} {issues.length === 1 ? "issue" : "issues"}</span>
          </div>
          {error && <p className="notice" role="alert">{error}</p>}
          {selectedProject ? (
            <>
              <form className="issue-form" onSubmit={createIssue}>
                <div className="form-heading"><strong>Create an issue</strong><span>Capture the next thing to move forward</span></div>
                <label htmlFor="issue-title">Title</label>
                <input id="issue-title" name="title" placeholder="What needs doing?" maxLength={200} required />
                <label htmlFor="issue-description">Description <span className="optional">(optional)</span></label>
                <textarea id="issue-description" name="description" rows={3} placeholder="Add context, acceptance criteria, or links." />
                <button className="primary-button">Add issue <span aria-hidden="true">→</span></button>
              </form>
              <div className="issue-list" aria-live="polite">
                {issues.length === 0 ? <div className="empty-state"><span className="empty-icon">✳</span><h2>A clean slate</h2><p>Create your first issue to get moving.</p></div> : issues.map((issue) => (
                  <article className="issue-card" key={issue.id}>
                    <div className="issue-topline">
                      <span className={`status-dot ${issue.status}`} aria-hidden="true" />
                      <h2>{issue.title}</h2>
                      <select aria-label={`Status for ${issue.title}`} value={issue.status} onChange={(event) => void changeStatus(issue, event.target.value as Issue["status"])}>
                        {statuses.map((status) => <option key={status} value={status}>{status.replace("_", " ")}</option>)}
                      </select>
                    </div>
                    {issue.description && <p className="issue-description">{issue.description}</p>}
                    <div className="issue-footer">
                      <span>Added {new Date(issue.createdAt).toLocaleDateString()}</span>
                      <button className="link-button" onClick={() => void toggleComments(issue.id)}>{openComments === issue.id ? "Hide discussion" : "Discussion"}</button>
                    </div>
                    {openComments === issue.id && <div className="comments">
                      {comments.length === 0 ? <p className="muted">No comments yet. Start the conversation.</p> : comments.map((comment) => <p className="comment" key={comment.id}>{comment.body}</p>)}
                      <form className="comment-form" onSubmit={(event) => void createComment(event, issue.id)}>
                        <input name="body" placeholder="Write a comment…" maxLength={4000} required aria-label="Write a comment" />
                        <button className="secondary">Send</button>
                      </form>
                    </div>}
                  </article>
                ))}
              </div>
            </>
          ) : <div className="empty-state"><span className="empty-icon">＋</span><h2>Start with a project</h2><p>Create a project in the sidebar to collect its issues.</p></div>}
        </div>
      </section>
    </main>
  );
}

export default App;
