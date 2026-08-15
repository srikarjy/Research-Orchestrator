// Client-side auth: token storage plus the gateway's /api/v1/auth endpoints.
// The gateway may run with auth disabled (local dev, no ORCH_AUTH_JWT_SECRET);
// the UI treats auth as required only after a request actually 401s, so a
// dev stack without a secret never shows a login wall.

import { apiConfig } from "./client";

const TOKEN_KEY = "ro.auth.token";
const EMAIL_KEY = "ro.auth.email";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function getEmail(): string | null {
  return localStorage.getItem(EMAIL_KEY);
}

export function setSession(token: string, email: string) {
  localStorage.setItem(TOKEN_KEY, token);
  localStorage.setItem(EMAIL_KEY, email);
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(EMAIL_KEY);
}

/** Fired by the API layer when a request 401s, so the app can show login. */
export const AUTH_REQUIRED_EVENT = "ro:auth-required";

interface AuthResponse {
  token: string;
  user: { id: string; email: string };
}

async function authPost(path: string, email: string, password: string): Promise<AuthResponse> {
  const res = await fetch(`${apiConfig.base}/api/v1/auth/${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(body.error || `${path} failed (${res.status})`);
  }
  return body as AuthResponse;
}

export async function login(email: string, password: string): Promise<AuthResponse> {
  const out = await authPost("login", email, password);
  setSession(out.token, out.user.email);
  return out;
}

export async function register(email: string, password: string): Promise<AuthResponse> {
  const out = await authPost("register", email, password);
  setSession(out.token, out.user.email);
  return out;
}
