import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Card, Button, InputGroup, Callout, FormGroup } from "@blueprintjs/core";
import { api } from "../../api/client";

export function RegisterPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username || !email) {
      setError("Please complete all fields.");
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      setMessage(null);

      const response = await api<{ success: boolean; message?: string; error?: string }>("/api/v1/register", {
        method: "POST",
        body: JSON.stringify({
          username,
          email,
        }),
      });

      if (response.success) {
        setMessage(response.message || "Registration successful! Check your email for login details.");
        setUsername("");
        setEmail("");
      } else {
        setError(response.error || "Registration failed.");
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
            Create Account
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

        {message && (
          <Callout
            intent="success"
            style={{
              borderRadius: 0,
              marginBottom: "1.5rem",
              fontFamily: "Space Mono, monospace",
              fontSize: "0.8rem",
            }}
          >
            {message}
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
              placeholder="Enter desired username"
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
            label="Email Address"
            labelFor="email-input"
            labelInfo="(required)"
            style={{
              fontFamily: "Space Mono, monospace",
              textTransform: "uppercase",
              fontSize: "0.75rem",
              marginBottom: "2rem",
            }}
          >
            <InputGroup
              id="email-input"
              type="email"
              placeholder="Enter your email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              large
              style={{
                borderRadius: 0,
                border: "1px solid var(--border-color)",
                fontFamily: "sans-serif",
              }}
            />
          </FormGroup>

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
            Register
          </Button>
        </form>

        <div style={{ textAlign: "center", marginTop: "2rem", fontSize: "0.8rem" }}>
          <span style={{ color: "var(--text-secondary)" }}>Already have an account? </span>
          <Button
            variant="minimal"
            onClick={() => navigate("/spa")}
            style={{
              color: "var(--accent-primary)",
              fontFamily: "Space Mono, monospace",
              textTransform: "uppercase",
              padding: "0 4px",
              minHeight: 0,
            }}
          >
            Sign In
          </Button>
        </div>
      </Card>
    </div>
  );
}
