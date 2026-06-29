import {
  Navbar,
  NavbarGroup,
  NavbarHeading,
  NavbarDivider,
  Alignment,
  Button,
  HTMLSelect,
  Label,
  InputGroup,
  Popover,
  Menu,
  MenuItem,
  Dialog,
  Spinner,
  HTMLTable,
  MenuDivider,
} from "@blueprintjs/core";
import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { useState, useRef } from "react";
import { useApp } from "../context/AppContext";
import { api } from "../api/client";
import { useDatasets, useStats } from "../hooks/useDatasets";
import { UploadManager, UploadManagerRef } from "./UploadManager";

export function Layout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, refetchSession } = useApp();
  const [searchVal, setSearchVal] = useState("");
  const [isStatsOpen, setIsStatsOpen] = useState(false);
  const uploadManagerRef = useRef<UploadManagerRef>(null);

  const canUpload = !!(user?.role_admin || user?.role_upload);
  const [isInfoOpen, setIsInfoOpen] = useState(false);

  // Fetch datasets list for quick-switch dropdown
  const { data: datasets } = useDatasets();

  // Fetch stats data
  const { data: stats, isLoading: isStatsLoading } = useStats();

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchVal.trim()) {
      navigate(`/spa?q=${encodeURIComponent(searchVal)}`);
    }
  };

  const handleLogout = async () => {
    try {
      await api("/api/v1/logout", { method: "POST" });
      await refetchSession();
      navigate("/spa");
    } catch (err) {
      console.error("Logout failed", err);
      window.location.href = "/logout";
    }
  };

  // Extract sort/order/limit from URL to keep UI in sync
  const queryParams = new URLSearchParams(location.search);
  const sortBy = queryParams.get("sort") || "timestamp";
  const order = queryParams.get("order") || "desc";
  const limit = queryParams.get("limit") || "60";
  const coverSize = queryParams.get("cover") || "md";

  const updateParam = (key: string, value: string) => {
    const params = new URLSearchParams(location.search);
    params.set(key, value);
    // Reset offset when sort options change
    params.set("offset", "0");
    navigate(`${location.pathname}?${params.toString()}`);
  };

  // cover size doesn't change row count, so preserve the current page offset.
  const updateCoverSize = (value: string) => {
    const params = new URLSearchParams(location.search);
    params.set("cover", value);
    navigate(`${location.pathname}?${params.toString()}`);
  };

  return (
    <>
      <Navbar className="bp6-navbar">
        <NavbarGroup align={Alignment.START}>
          <NavbarHeading className="bp6-navbar-heading" style={{ cursor: "pointer" }} onClick={() => navigate("/spa")}>
            Qalibre
          </NavbarHeading>
          <NavbarDivider />
          <Button
            icon="home"
            variant="minimal"
            text="Books"
            onClick={() => navigate("/spa")}
            active={location.pathname === "/spa"}
            style={{ fontFamily: "var(--font-mono)", fontSize: "0.75rem", textTransform: "uppercase" }}
          />
          <Popover
            content={
              <Menu>
                {datasets && datasets.map((d) => (
                  <MenuItem
                    key={d.id}
                    icon="database"
                    text={d.name}
                    onClick={() => navigate(`/spa/datasets/${d.id}`)}
                    active={location.pathname === `/spa/datasets/${d.id}`}
                  />
                ))}
                {(!datasets || datasets.length === 0) && (
                  <MenuItem disabled text="No datasets found" />
                )}
                <MenuDivider />
                <MenuItem
                  icon="cog"
                  text="Manage Datasets"
                  onClick={() => navigate("/spa/datasets")}
                />
              </Menu>
            }
            placement="bottom-start"
          >
            <Button
              icon="database"
              variant="minimal"
              text="Datasets"
              rightIcon="caret-down"
              active={location.pathname.startsWith("/spa/datasets")}
              style={{ fontFamily: "var(--font-mono)", fontSize: "0.75rem", textTransform: "uppercase" }}
            />
          </Popover>
          <Button
            icon="trending-up"
            variant="minimal"
            text="Stats"
            onClick={() => setIsStatsOpen(true)}
            style={{ fontFamily: "var(--font-mono)", fontSize: "0.75rem", textTransform: "uppercase" }}
          />
          <Button
            icon="info-sign"
            variant="minimal"
            text="Info"
            onClick={() => setIsInfoOpen(true)}
            style={{ fontFamily: "var(--font-mono)", fontSize: "0.75rem", textTransform: "uppercase" }}
          />
        </NavbarGroup>
        <NavbarGroup align={Alignment.END}>
          <form onSubmit={handleSearchSubmit}>
            <InputGroup
              leftIcon="search"
              placeholder="Search..."
              value={searchVal}
              onChange={(e) => setSearchVal(e.target.value)}
            />
          </form>
          <NavbarDivider />
          <Button
            icon="cog"
            variant="minimal"
            onClick={() => navigate("/spa/config")}
            title="Configuration"
          />
          <Button
            icon="user"
            variant="minimal"
            onClick={() => navigate("/spa/admin")}
            title="Admin Panel"
          />
          {user ? (
            <Button
              icon="log-out"
              variant="minimal"
              onClick={handleLogout}
              title="Logout"
            />
          ) : (
            <Button
              icon="log-in"
              variant="minimal"
              onClick={() => navigate("/spa/login")}
              title="Login"
            />
          )}
        </NavbarGroup>
      </Navbar>


      <div className="app-layout">
        <aside className="sidebar">
          <div style={{ flex: 1 }}>
            <h6
              className="bp6-label"
              style={{
                fontFamily: "var(--font-mono)",
                fontSize: "0.75rem",
                textTransform: "uppercase",
                letterSpacing: "0.15em",
                fontWeight: 400,
                color: "var(--text-primary)",
                marginBottom: "1rem",
                marginTop: 0,
              }}
            >
              Browse
            </h6>
            <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem", marginBottom: "2rem" }}>
              <Button
                alignText="left"
                icon="book"
                variant="minimal"
                fill
                onClick={() => navigate("/spa")}
                active={location.pathname === "/spa"}
              >
                All Books
              </Button>
              <Button
                alignText="left"
                icon="people"
                variant="minimal"
                fill
                onClick={() => navigate("/spa/authors")}
                active={location.pathname === "/spa/authors"}
              >
                Authors
              </Button>
              <Button
                alignText="left"
                icon="properties"
                variant="minimal"
                fill
                onClick={() => navigate("/spa/series")}
                active={location.pathname === "/spa/series"}
              >
                Series
              </Button>
              <Button
                alignText="left"
                icon="database"
                variant="minimal"
                fill
                onClick={() => navigate("/spa/datasets")}
                active={location.pathname.startsWith("/spa/datasets")}
              >
                Datasets
              </Button>
              <Button
                alignText="left"
                icon="office"
                variant="minimal"
                fill
                onClick={() => navigate("/spa/publishers")}
                active={location.pathname === "/spa/publishers"}
              >
                Publishers
              </Button>
              <Button
                alignText="left"
                icon="tag"
                variant="minimal"
                fill
                onClick={() => navigate("/spa/categories")}
                active={location.pathname === "/spa/categories"}
              >
                Categories
              </Button>
              <Button
                alignText="left"
                icon="star"
                variant="minimal"
                fill
                onClick={() => navigate("/spa/shelves")}
                active={location.pathname.startsWith("/spa/shelves")}
              >
                Shelves
              </Button>
              {canUpload && (
                <Button
                  alignText="left"
                  icon="upload"
                  variant="minimal"
                  fill
                  onClick={() => uploadManagerRef.current?.triggerUpload()}
                >
                  Upload Books
                </Button>
              )}
            </div>
          </div>

          <div className="sort-controls" style={{ borderTop: "1px solid var(--border-light)", paddingTop: "1.5rem" }}>
            <Label className="bp6-label">
              Sort By
              <HTMLSelect
                fill
                value={sortBy}
                onChange={(e) => updateParam("sort", e.target.value)}
              >
                <option value="title">Title</option>
                <option value="authors">Author</option>
                <option value="pubdate">Published Date</option>
                <option value="timestamp">Date Added</option>
                <option value="rating">Rating</option>
              </HTMLSelect>
            </Label>

            <Label className="bp6-label" style={{ marginTop: "1rem" }}>
              Order
              <HTMLSelect
                fill
                value={order}
                onChange={(e) => updateParam("order", e.target.value)}
              >
                <option value="asc">Ascending</option>
                <option value="desc">Descending</option>
              </HTMLSelect>
            </Label>

            <Label className="bp6-label" style={{ marginTop: "1rem" }}>
              Page Size
              <HTMLSelect
                fill
                value={limit}
                onChange={(e) => updateParam("limit", e.target.value)}
              >
                <option value="12">12 books</option>
                <option value="24">24 books</option>
                <option value="60">60 books</option>
                <option value="120">120 books</option>
              </HTMLSelect>
            </Label>

            <Label className="bp6-label" style={{ marginTop: "1rem" }}>
              Cover Size
              <HTMLSelect
                fill
                value={coverSize}
                onChange={(e) => updateCoverSize(e.target.value)}
              >
                <option value="sm">Small</option>
                <option value="md">Medium</option>
                <option value="lg">Large</option>
              </HTMLSelect>
            </Label>
          </div>
        </aside>

        <main className="main-content">
          <Outlet />
        </main>
      </div>

      {/* Stats Dialog */}
      <Dialog
        isOpen={isStatsOpen}
        onClose={() => setIsStatsOpen(false)}
        title="System Statistics"
        icon="trending-up"
        style={{
          borderRadius: 0,
          backgroundColor: "var(--bg-secondary)",
          width: "90%",
          maxWidth: "600px",
          color: "var(--text-primary)",
        }}
      >
        <div style={{ padding: "1.5rem" }}>
          {isStatsLoading ? (
            <div style={{ display: "flex", justifyContent: "center", padding: "2rem" }}>
              <Spinner size={30} />
            </div>
          ) : !stats ? (
            <p>Failed to load statistics.</p>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: "1.5rem" }}>
              {/* General Stats Table */}
              <HTMLTable style={{ width: "100%" }} bordered striped interactive>
                <thead>
                  <tr>
                    <th>Metric</th>
                    <th>Count</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td>Total Books</td>
                    <td>{stats.total_books}</td>
                  </tr>
                  <tr>
                    <td>Total Authors</td>
                    <td>{stats.total_authors}</td>
                  </tr>
                  <tr>
                    <td>Total Series</td>
                    <td>{stats.total_series}</td>
                  </tr>
                  <tr>
                    <td>Total Tags</td>
                    <td>{stats.total_tags}</td>
                  </tr>
                  <tr>
                    <td>Total Publishers</td>
                    <td>{stats.total_publishers}</td>
                  </tr>
                  <tr>
                    <td>Books Read (You)</td>
                    <td>{stats.read_books}</td>
                  </tr>
                  <tr>
                    <td>Books Unread (You)</td>
                    <td>{stats.unread_books}</td>
                  </tr>
                  <tr>
                    <td>Books Archived (You)</td>
                    <td>{stats.archived_books}</td>
                  </tr>
                </tbody>
              </HTMLTable>

              {/* Format Stats Table */}
              <div>
                <h5 style={{ marginBottom: "0.5rem" }}>Formats Distribution</h5>
                <HTMLTable style={{ width: "100%" }} bordered striped interactive>
                  <thead>
                    <tr>
                      <th>Format</th>
                      <th>Books Count</th>
                      <th>Total Size</th>
                    </tr>
                  </thead>
                  <tbody>
                    {stats.formats && stats.formats.map((f) => (
                      <tr key={f.format}>
                        <td>{f.format}</td>
                        <td>{f.count}</td>
                        <td>{(f.size / (1024 * 1024)).toFixed(2)} MB</td>
                      </tr>
                    ))}
                    {(!stats.formats || stats.formats.length === 0) && (
                      <tr>
                        <td colSpan={3} style={{ textAlign: "center" }}>No formats found</td>
                      </tr>
                    )}
                  </tbody>
                </HTMLTable>
              </div>
            </div>
          )}
        </div>
      </Dialog>

      {/* Info Dialog */}
      <Dialog
        isOpen={isInfoOpen}
        onClose={() => setIsInfoOpen(false)}
        title="About Qalibre"
        icon="info-sign"
        style={{
          borderRadius: 0,
          backgroundColor: "var(--bg-secondary)",
          width: "90%",
          maxWidth: "500px",
          color: "var(--text-primary)",
        }}
      >
        <div style={{ padding: "1.5rem", display: "flex", flexDirection: "column", gap: "1rem" }}>
          <div style={{ display: "flex", flexDirection: "column", alignItems: "center", marginBottom: "0.5rem" }}>
            <img
              src="/logo.png"
              alt="Qalibre Logo"
              width="140"
              style={{ borderRadius: 0, boxShadow: "0 4px 10px rgba(0, 0, 0, 0.15)", marginBottom: "1rem" }}
            />
            <h4 style={{ margin: "0 0 0.5rem 0", textAlign: "center" }}>Qalibre Document Manager</h4>
            <p style={{ margin: 0, fontSize: "0.9rem", color: "var(--text-secondary)", textAlign: "center" }}>
              A high-performance document database manager built for AI dataset preparation and library curation.
            </p>
          </div>
          
          <div style={{ borderTop: "1px solid var(--border-light)", paddingTop: "1rem" }}>
            <p style={{ margin: "0 0 0.5rem 0" }}>
              <strong>Version:</strong> {stats?.version && stats.version !== "unspecified" ? stats.version : "0.7.0-go (development)"}
            </p>
            <p style={{ margin: "0 0 0.5rem 0" }}>
              <strong>Author:</strong> Developed and ported to Go by <a href="https://github.com/mamorett" target="_blank" rel="noopener noreferrer">@mamorett</a>
            </p>
            <p style={{ margin: "0 0 0.5rem 0" }}>
              <strong>License:</strong> GPL v3 (inspired by Calibre-Web, rebuilt in Go & React)
            </p>
            <p style={{ margin: 0 }}>
              <strong>Copyright:</strong> &copy; {new Date().getFullYear()} mamorett. All rights reserved.
            </p>
          </div>
        </div>
      </Dialog>
      <UploadManager ref={uploadManagerRef} />
    </>
  );
}
