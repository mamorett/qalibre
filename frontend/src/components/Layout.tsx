import {
  Navbar,
  NavbarGroup,
  NavbarHeading,
  NavbarDivider,
  Alignment,
  Button,
  HTMLSelect,
  Label,
  InputGroup
} from "@blueprintjs/core";
import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { useState } from "react";
import { useApp } from "../context/AppContext";
import { api } from "../api/client";

export function Layout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, refetchSession } = useApp();
  const [searchVal, setSearchVal] = useState("");

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
    </>
  );
}
