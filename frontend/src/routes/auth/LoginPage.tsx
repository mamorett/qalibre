import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Card, Button, InputGroup, Checkbox, Callout, FormGroup } from "@blueprintjs/core";
import { api } from "../../api/client";
import { useApp } from "../../context/AppContext";

export function LoginPage() {
  const navigate = useNavigate();
  const { config, refetchSession } = useApp();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [rememberMe, setRememberMe] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username || !password) {
      setError("Please enter both username and password.");
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      
      const response = await api<{ success: boolean; error?: string }>("/api/v1/login", {
        method: "POST",
        body: JSON.stringify({
          username,
          password,
          remember_me: rememberMe,
        }),
      });

      if (response.success) {
        await refetchSession();
        navigate("/spa");
      } else {
        setError(response.error || "Login failed.");
      }
    } catch (err: any) {
      setError(err?.message || "An unexpected error occurred. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div
      style={{
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        minHeight: "100vh",
        backgroundColor: "var(--bg-primary)",
        color: "var(--text-primary)",
        fontFamily: "Space Mono, monospace",
        padding: "1rem",
      }}
    >
      <Card
        style={{
          width: "100%",
          maxWidth: "420px",
          padding: "2.5rem 2rem",
          backgroundColor: "var(--bg-secondary)",
          border: "1px solid var(--border-color)",
          borderRadius: 0,
          boxShadow: "none",
        }}
      >
        <div style={{ textAlign: "center", marginBottom: "2rem" }}>
          <h1
            style={{
              fontFamily: "Playfair Display, Georgia, serif",
              fontWeight: 700,
              fontSize: "2.2rem",
              margin: "0 0 0.5rem 0",
              color: "var(--accent-primary)",
            }}
          >
            Qalibre
          </h1>
          <span style={{ fontSize: "0.8rem", textTransform: "uppercase", letterSpacing: "0.15em", color: "var(--text-secondary)" }}>
            Sign In
          </span>
        </div>

        {error && (
          <Callout
            intent="danger"
            style={{
              borderRadius: 0,
              marginBottom: "1.5rem",
              fontFamily: "Space Mono, monospace",
              fontSize: "0.8rem",
            }}
          >
            {error}
          </Callout>
        )}

        <form onSubmit={handleSubmit}>
          <FormGroup
            label="Username"
            labelFor="username-input"
            labelInfo="(required)"
            style={{
              fontFamily: "Space Mono, monospace",
              textTransform: "uppercase",
              fontSize: "0.75rem",
              marginBottom: "1.5rem",
            }}
          >
            <InputGroup
              id="username-input"
              placeholder="Enter your username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              large
              style={{
                borderRadius: 0,
                border: "1px solid var(--border-color)",
                fontFamily: "sans-serif",
              }}
            />
          </FormGroup>

          <FormGroup
            label="Password"
            labelFor="password-input"
            labelInfo="(required)"
            style={{
              fontFamily: "Space Mono, monospace",
              textTransform: "uppercase",
              fontSize: "0.75rem",
              marginBottom: "1.5rem",
            }}
          >
            <InputGroup
              id="password-input"
              type="password"
              placeholder="Enter your password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              large
              style={{
                borderRadius: 0,
                border: "1px solid var(--border-color)",
                fontFamily: "sans-serif",
              }}
            />
          </FormGroup>

          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              marginBottom: "2rem",
            }}
          >
            <Checkbox
              label="Remember me"
              checked={rememberMe}
              onChange={(e) => setRememberMe(e.target.checked)}
              style={{
                fontFamily: "Space Mono, monospace",
                fontSize: "0.8rem",
                textTransform: "uppercase",
                margin: 0,
              }}
            />
          </div>

          <Button
            type="submit"
            intent="primary"
            fill
            large
            loading={isLoading}
            style={{
              borderRadius: 0,
              fontFamily: "Space Mono, monospace",
              textTransform: "uppercase",
              fontWeight: "bold",
              letterSpacing: "0.1em",
              border: "1px solid var(--border-color)",
              backgroundColor: "transparent",
              color: "var(--text-primary)",
            }}
          >
            Sign In
          </Button>
        </form>

        {config?.public_register && (
          <div style={{ textAlign: "center", marginTop: "2rem", fontSize: "0.8rem" }}>
            <span style={{ color: "var(--text-secondary)" }}>Don't have an account? </span>
            <Button
              variant="minimal"
              onClick={() => navigate("/spa/register")}
              style={{
                color: "var(--accent-primary)",
                fontFamily: "Space Mono, monospace",
                textTransform: "uppercase",
                padding: "0 4px",
                minHeight: 0,
              }}
            >
              Register
            </Button>
          </div>
        )}
      </Card>
    </div>
  );
}
