import React, { useEffect, useState } from "react";
import { Card, FormGroup, InputGroup, Checkbox, Button, Callout, Spinner, H2 } from "@blueprintjs/core";
import { api } from "../../api/client";
import { useApp } from "../../context/AppContext";

export default function ConfigPage() {
  const { user, refetchSession } = useApp();
  const [configData, setConfigData] = useState({
    config_calibre_dir: "",
    config_books_per_page: 60,
    config_calibre_web_title: "Qalibre",
    config_public_reg: false,
    config_uploading: false,
    config_anonbrowse: false,
  });
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchConfig = async () => {
      try {
        setIsLoading(true);
        const data = await api<typeof configData>("/api/v1/config");
        setConfigData(data);
      } catch (err: any) {
        setError(err?.message || "Failed to load configuration.");
      } finally {
        setIsLoading(false);
      }
    };
    fetchConfig();
  }, []);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setIsSaving(true);
      setError(null);
      setMessage(null);
      await api("/api/v1/config", {
        method: "POST",
        body: JSON.stringify(configData),
      });
      setMessage("Configuration updated successfully.");
      await refetchSession();
    } catch (err: any) {
      setError(err?.message || "Failed to save configuration.");
    } finally {
      setIsSaving(false);
    }
  };

  if (!user?.role_admin) {
    return (
      <Callout intent="danger" title="Access Denied" style={{ borderRadius: 0 }}>
        You must be an administrator to view this page.
      </Callout>
    );
  }

  if (isLoading) {
    return (
      <div style={{ display: "flex", justifyContent: "center", padding: "3rem" }}>
        <Spinner size={40} />
      </div>
    );
  }

  return (
    <div style={{ maxWidth: "600px", margin: "0 auto", padding: "1rem" }}>
      <H2 style={{ fontFamily: "Playfair Display, serif", marginBottom: "2rem" }}>Basic Configuration</H2>

      {error && (
        <Callout intent="danger" style={{ borderRadius: 0, marginBottom: "1.5rem" }}>
          {error}
        </Callout>
      )}

      {message && (
        <Callout intent="success" style={{ borderRadius: 0, marginBottom: "1.5rem" }}>
          {message}
        </Callout>
      )}

      <Card style={{ borderRadius: 0, border: "1px solid var(--border-color)", backgroundColor: "var(--bg-secondary)", boxShadow: "none", padding: "2rem" }}>
        <form onSubmit={handleSave}>
          <FormGroup
            label="Location of Calibre Database"
            labelInfo="(folder containing metadata.db)"
            labelFor="db-dir-input"
            style={{ textTransform: "uppercase", fontSize: "0.75rem", fontFamily: "Space Mono, monospace", marginBottom: "1.5rem" }}
          >
            <InputGroup
              id="db-dir-input"
              value={configData.config_calibre_dir}
              onChange={(e) => setConfigData({ ...configData, config_calibre_dir: e.target.value })}
              placeholder="e.g. /library"
              large
              style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
            />
          </FormGroup>

          <FormGroup
            label="Site Title"
            labelFor="title-input"
            style={{ textTransform: "uppercase", fontSize: "0.75rem", fontFamily: "Space Mono, monospace", marginBottom: "1.5rem" }}
          >
            <InputGroup
              id="title-input"
              value={configData.config_calibre_web_title}
              onChange={(e) => setConfigData({ ...configData, config_calibre_web_title: e.target.value })}
              large
              style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
            />
          </FormGroup>

          <FormGroup
            label="Books Per Page"
            labelFor="books-per-page-input"
            style={{ textTransform: "uppercase", fontSize: "0.75rem", fontFamily: "Space Mono, monospace", marginBottom: "2rem" }}
          >
            <InputGroup
              id="books-per-page-input"
              type="number"
              value={String(configData.config_books_per_page)}
              onChange={(e) => setConfigData({ ...configData, config_books_per_page: parseInt(e.target.value) || 60 })}
              large
              style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
            />
          </FormGroup>

          <div style={{ display: "flex", flexDirection: "column", gap: "1rem", marginBottom: "2.5rem" }}>
            <Checkbox
              label="Allow Anonymous Browsing"
              checked={configData.config_anonbrowse}
              onChange={(e) => setConfigData({ ...configData, config_anonbrowse: e.target.checked })}
              style={{ fontFamily: "Space Mono, monospace", fontSize: "0.8rem", textTransform: "uppercase" }}
            />
            <Checkbox
              label="Allow Public Registration"
              checked={configData.config_public_reg}
              onChange={(e) => setConfigData({ ...configData, config_public_reg: e.target.checked })}
              style={{ fontFamily: "Space Mono, monospace", fontSize: "0.8rem", textTransform: "uppercase" }}
            />
            <Checkbox
              label="Enable Book Uploads"
              checked={configData.config_uploading}
              onChange={(e) => setConfigData({ ...configData, config_uploading: e.target.checked })}
              style={{ fontFamily: "Space Mono, monospace", fontSize: "0.8rem", textTransform: "uppercase" }}
            />
          </div>

          <Button
            type="submit"
            intent="primary"
            large
            fill
            loading={isSaving}
            style={{
              borderRadius: 0,
              fontFamily: "Space Mono, monospace",
              textTransform: "uppercase",
              fontWeight: "bold",
              letterSpacing: "0.1em",
              border: "1px solid var(--border-color)",
              backgroundColor: "transparent",
              color: "var(--text-primary)"
            }}
          >
            Save Changes
          </Button>
        </form>
      </Card>
    </div>
  );
}
