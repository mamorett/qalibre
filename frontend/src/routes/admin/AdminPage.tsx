import React, { useEffect, useState } from "react";
import { Card, FormGroup, InputGroup, Button, Callout, Spinner, H2, Dialog, DialogBody, DialogFooter } from "@blueprintjs/core";
import { api } from "../../api/client";
import { useApp } from "../../context/AppContext";

interface UserInfo {
  id: number;
  name: string;
  email: string;
  role_admin: boolean;
  role_edit: boolean;
  role_download: boolean;
  role_upload: boolean;
}

export default function AdminPage() {
  const { user } = useApp();
  const [users, setUsers] = useState<UserInfo[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  // Dialog state for adding a user
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [newUsername, setNewUsername] = useState("");
  const [newEmail, setNewEmail] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const fetchUsers = async () => {
    try {
      setIsLoading(true);
      const data = await api<UserInfo[]>("/api/v1/admin/users");
      setUsers(data);
    } catch (err: any) {
      setError(err?.message || "Failed to load user list.");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  const handleDeleteUser = async (userId: number, username: string) => {
    if (!window.confirm(`Are you sure you want to delete user "${username}"?`)) {
      return;
    }
    try {
      setError(null);
      setMessage(null);
      await api(`/api/v1/admin/users/${userId}`, { method: "DELETE" });
      setMessage(`User "${username}" deleted successfully.`);
      fetchUsers();
    } catch (err: any) {
      setError(err?.message || `Failed to delete user "${username}".`);
    }
  };

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUsername || !newPassword) {
      setError("Username and password are required.");
      return;
    }
    try {
      setIsCreating(true);
      setError(null);
      setMessage(null);
      await api("/api/v1/admin/users", {
        method: "POST",
        body: JSON.stringify({
          username: newUsername,
          email: newEmail,
          password: newPassword,
        }),
      });
      setMessage(`User "${newUsername}" created successfully.`);
      setIsDialogOpen(false);
      setNewUsername("");
      setNewEmail("");
      setNewPassword("");
      fetchUsers();
    } catch (err: any) {
      setError(err?.message || "Failed to create user.");
    } finally {
      setIsCreating(false);
    }
  };

  if (!user?.role_admin) {
    return (
      <Callout intent="danger" title="Access Denied" style={{ borderRadius: 0 }}>
        You must be an administrator to view this page.
      </Callout>
    );
  }

  return (
    <div style={{ maxWidth: "800px", margin: "0 auto", padding: "1rem" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "2rem" }}>
        <H2 style={{ fontFamily: "Playfair Display, serif", margin: 0 }}>User Management</H2>
        <Button
          icon="plus"
          onClick={() => setIsDialogOpen(true)}
          style={{
            borderRadius: 0,
            fontFamily: "Space Mono, monospace",
            textTransform: "uppercase",
            border: "1px solid var(--border-color)",
            backgroundColor: "transparent",
            color: "var(--text-primary)"
          }}
        >
          Add User
        </Button>
      </div>

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

      {isLoading ? (
        <div style={{ display: "flex", justifyContent: "center", padding: "3rem" }}>
          <Spinner size={40} />
        </div>
      ) : (
        <Card style={{ borderRadius: 0, border: "1px solid var(--border-color)", backgroundColor: "var(--bg-secondary)", boxShadow: "none", padding: 0 }}>
          <table className="bp6-html-table bp6-html-table-striped" style={{ width: "100%", textAlign: "left", borderCollapse: "collapse" }}>
            <thead>
              <tr style={{ fontFamily: "Space Mono, monospace", fontSize: "0.75rem", textTransform: "uppercase", borderBottom: "2px solid var(--border-color)" }}>
                <th style={{ padding: "1rem" }}>Username</th>
                <th style={{ padding: "1rem" }}>Email</th>
                <th style={{ padding: "1rem" }}>Admin</th>
                <th style={{ padding: "1rem", textAlign: "right" }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} style={{ borderBottom: "1px solid var(--border-light)" }}>
                  <td style={{ padding: "1rem", fontWeight: "bold" }}>{u.name}</td>
                  <td style={{ padding: "1rem" }}>{u.email || "N/A"}</td>
                  <td style={{ padding: "1rem" }}>{u.role_admin ? "Yes" : "No"}</td>
                  <td style={{ padding: "1rem", textAlign: "right" }}>
                    <Button
                      icon="trash"
                      intent="danger"
                      variant="minimal"
                      disabled={u.id === user.id}
                      onClick={() => handleDeleteUser(u.id, u.name)}
                      title={u.id === user.id ? "Cannot delete yourself" : "Delete User"}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}

      <Dialog
        isOpen={isDialogOpen}
        onClose={() => setIsDialogOpen(false)}
        title="Add New User"
        style={{ borderRadius: 0, fontFamily: "Space Mono, monospace", backgroundColor: "var(--bg-secondary)", border: "1px solid var(--border-color)" }}
      >
        <form onSubmit={handleCreateUser}>
          <DialogBody>
            <FormGroup
              label="Username"
              labelInfo="(required)"
              labelFor="new-username"
              style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem" }}
            >
              <InputGroup
                id="new-username"
                value={newUsername}
                onChange={(e) => setNewUsername(e.target.value)}
                style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
              />
            </FormGroup>

            <FormGroup
              label="Email"
              labelFor="new-email"
              style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem" }}
            >
              <InputGroup
                id="new-email"
                type="email"
                value={newEmail}
                onChange={(e) => setNewEmail(e.target.value)}
                style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
              />
            </FormGroup>

            <FormGroup
              label="Password"
              labelInfo="(required)"
              labelFor="new-password"
              style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem" }}
            >
              <InputGroup
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
              />
            </FormGroup>
          </DialogBody>
          <DialogFooter
            actions={
              <div style={{ display: "flex", gap: "1rem" }}>
                <Button
                  onClick={() => setIsDialogOpen(false)}
                  style={{ borderRadius: 0, fontFamily: "Space Mono, monospace", textTransform: "uppercase" }}
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  intent="primary"
                  loading={isCreating}
                  style={{ borderRadius: 0, fontFamily: "Space Mono, monospace", textTransform: "uppercase" }}
                >
                  Create
                </Button>
              </div>
            }
          />
        </form>
      </Dialog>
    </div>
  );
}
