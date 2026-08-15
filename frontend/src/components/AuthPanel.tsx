import { useState } from "react";
import { login, register } from "../api/auth";
import "./AuthPanel.css";

interface AuthPanelProps {
  onAuthed: (email: string) => void;
  onDismiss: () => void;
}

export function AuthPanel({ onAuthed, onDismiss }: AuthPanelProps) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const out = mode === "login" ? await login(email, password) : await register(email, password);
      onAuthed(out.user.email);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Authentication failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="auth-panel__backdrop" role="dialog" aria-modal="true" aria-label="Sign in">
      <div className="auth-panel">
        <div className="auth-panel__header">
          <span className="auth-panel__title">
            {mode === "login" ? "Sign in" : "Create account"}
          </span>
          <button className="auth-panel__close" onClick={onDismiss} aria-label="Close">
            ×
          </button>
        </div>

        <form onSubmit={submit} className="auth-panel__form">
          <label className="auth-panel__label">
            Email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
              className="auth-panel__input"
            />
          </label>
          <label className="auth-panel__label">
            Password
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              autoComplete={mode === "login" ? "current-password" : "new-password"}
              className="auth-panel__input"
            />
          </label>

          {error && <div className="auth-panel__error">{error}</div>}

          <button type="submit" disabled={busy} className="auth-panel__submit">
            {busy ? "…" : mode === "login" ? "Sign in" : "Register"}
          </button>
        </form>

        <button
          className="auth-panel__switch"
          onClick={() => {
            setMode(mode === "login" ? "register" : "login");
            setError(null);
          }}
        >
          {mode === "login" ? "New here? Create an account" : "Have an account? Sign in"}
        </button>
      </div>
    </div>
  );
}
