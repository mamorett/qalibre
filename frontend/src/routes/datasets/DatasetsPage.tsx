import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Card,
  Button,
  Callout,
  Spinner,
  H2,
  H5,
  InputGroup,
  NonIdealState,
} from "@blueprintjs/core";
import { useApp } from "../../context/AppContext";
import { useDatasets } from "../../hooks/useDatasets";
import { DatasetFormDialog } from "../../components/DatasetFormDialog";
import { DatasetDetail } from "../../types/dataset";

export default function DatasetsPage() {
  const navigate = useNavigate();
  const { user } = useApp();
  const [search, setSearch] = useState("");
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  const { data: datasets, isLoading, isError, refetch } = useDatasets(search);

  if (!user?.role_admin) {
    return (
      <Callout intent="danger" title="Access Denied" style={{ borderRadius: 0, margin: "1rem" }}>
        You must be an administrator to view this page.
      </Callout>
    );
  }

  const handleCreateSuccess = (newDataset: DatasetDetail) => {
    navigate(`/spa/datasets/${newDataset.id}`);
  };

  return (
    <div style={{ padding: "1.5rem", minHeight: "100%", display: "flex", flexDirection: "column" }}>
      {/* Header section */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: "1.5rem",
          flexWrap: "wrap",
          gap: "1rem",
        }}
      >
        <div>
          <H2 style={{ fontFamily: "Playfair Display, serif", margin: 0, color: "var(--accent-primary)" }}>
            AI Datasets
          </H2>
          <p style={{ color: "var(--text-secondary)", margin: "0.25rem 0 0 0", fontSize: "0.9rem" }}>
            Group and export book text to Markdown for LLM fine-tuning and AI training.
          </p>
        </div>
        <Button
          icon="plus"
          intent="primary"
          onClick={() => setIsDialogOpen(true)}
          style={{
            borderRadius: 0,
            fontFamily: "Space Mono, monospace",
            textTransform: "uppercase",
          }}
        >
          New Dataset
        </Button>
      </div>

      {/* Search Input */}
      <div style={{ marginBottom: "1.5rem", maxWidth: "400px" }}>
        <InputGroup
          leftIcon="search"
          placeholder="Search datasets by name or description..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={{ borderRadius: 0, border: "1px solid var(--border-color)" }}
        />
      </div>

      {/* Content area */}
      {isLoading ? (
        <div style={{ display: "flex", justifyContent: "center", padding: "3rem", flex: 1, alignItems: "center" }}>
          <Spinner size={50} intent="primary" />
        </div>
      ) : isError ? (
        <NonIdealState
          icon="error"
          title="Failed to Load Datasets"
          description="There was an error communicating with the Qalibre server."
          action={<Button icon="refresh" onClick={() => refetch()}>Retry</Button>}
        />
      ) : !datasets || datasets.length === 0 ? (
        <NonIdealState
          icon="database"
          title={search ? "No Matching Datasets" : "No Datasets Created"}
          description={
            search
              ? `No datasets matched "${search}"`
              : "Group your books into custom datasets to prepare them for AI applications."
          }
          action={
            !search ? (
              <Button
                icon="plus"
                intent="primary"
                onClick={() => setIsDialogOpen(true)}
                style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
              >
                Create First Dataset
              </Button>
            ) : undefined
          }
        />
      ) : (
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))",
            gap: "1.5rem",
            alignContent: "start",
          }}
        >
          {datasets.map((dataset) => (
            <Card
              key={dataset.id}
              interactive
              onClick={() => navigate(`/spa/datasets/${dataset.id}`)}
              style={{
                borderRadius: 0,
                border: "1px solid var(--border-color)",
                display: "flex",
                flexDirection: "column",
                justifyContent: "space-between",
                padding: "1.25rem",
                height: "180px",
                backgroundColor: "var(--bg-secondary)",
              }}
            >
              <div>
                <H5 style={{ margin: "0 0 0.5rem 0", color: "var(--text-primary)" }}>{dataset.name}</H5>
                <p
                  style={{
                    color: "var(--text-secondary)",
                    fontSize: "0.85rem",
                    margin: 0,
                    overflow: "hidden",
                    display: "-webkit-box",
                    WebkitLineClamp: 3,
                    WebkitBoxOrient: "vertical",
                  }}
                >
                  {dataset.description || "No description provided."}
                </p>
              </div>
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  fontSize: "0.75rem",
                  color: "var(--text-muted)",
                  borderTop: "1px solid var(--border-light)",
                  paddingTop: "0.5rem",
                  marginTop: "0.5rem",
                  fontFamily: "Space Mono, monospace",
                }}
              >
                <span>{dataset.book_count} Books</span>
                <span>Mod: {new Date(dataset.last_modified).toLocaleDateString()}</span>
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Dialog for creation */}
      <DatasetFormDialog
        isOpen={isDialogOpen}
        mode="create"
        onClose={() => setIsDialogOpen(false)}
        onSuccess={handleCreateSuccess}
      />
    </div>
  );
}
