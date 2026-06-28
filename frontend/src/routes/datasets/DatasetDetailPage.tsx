import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  Card,
  Button,
  Callout,
  Spinner,
  H2,
  H3,
  H5,
  InputGroup,
  HTMLTable,
  NonIdealState,
  ProgressBar,
  Intent,
  Dialog,
  HTMLSelect,
} from "@blueprintjs/core";
import { useApp } from "../../context/AppContext";
import {
  useDataset,
  useDatasetBooks,
  useAvailableBooks,
  useDeleteDataset,
  useAddBooksToDataset,
  useRemoveBookFromDataset,
  useExportDataset,
  useTasksStatus,
  useCancelTask,
} from "../../hooks/useDatasets";
import { RowCard } from "../../components/RowCard";
import { Pagination } from "../../components/Pagination";
import { DatasetFormDialog } from "../../components/DatasetFormDialog";

export default function DatasetDetailPage() {
  const { id: idParam } = useParams<{ id: string }>();
  const datasetId = parseInt(idParam || "", 10);
  const navigate = useNavigate();
  const { user } = useApp();
  const queryClient = useQueryClient();

  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [exportPath, setExportPath] = useState("");
  const localStorageKey = `qalibre_active_export_task_${datasetId}`;
  const [activeTaskId, setActiveTaskId] = useState<string | null>(() => {
    return localStorage.getItem(localStorageKey) || null;
  });

  const updateActiveTaskId = (id: string | null) => {
    setActiveTaskId(id);
    if (id) {
      localStorage.setItem(localStorageKey, id);
    } else {
      localStorage.removeItem(localStorageKey);
    }
  };

  const [showAddBooks, setShowAddBooks] = useState(false);

  // Member books pagination
  const [memberOffset, setMemberOffset] = useState(0);
  const memberLimit = 10;

  // Available books search & pagination
  const [availableOffset, setAvailableOffset] = useState(0);
  const [availableSearch, setAvailableSearch] = useState("");
  const [availableLimit, setAvailableLimit] = useState(10);
  const [selectedBookIds, setSelectedBookIds] = useState<Set<number>>(new Set());

  // Queries
  const { data: dataset, isLoading: isDatasetLoading, isError: isDatasetError, refetch: refetchDataset } = useDataset(datasetId);
  const { data: memberBooksData, isLoading: isMemberBooksLoading } = useDatasetBooks(datasetId, {
    offset: memberOffset,
    limit: memberLimit,
  });
  const { data: availableBooksData, isLoading: isAvailableBooksLoading } = useAvailableBooks(datasetId, {
    offset: availableOffset,
    limit: availableLimit,
    search: availableSearch,
  });

  // Task polling hook
  const { data: tasks } = useTasksStatus(activeTaskId ? 1000 : 5000);

  // Mutations
  const deleteMutation = useDeleteDataset();
  const addBooksMutation = useAddBooksToDataset(datasetId);
  const removeBookMutation = useRemoveBookFromDataset(datasetId);
  const exportMutation = useExportDataset(datasetId);
  const cancelTaskMutation = useCancelTask();

  if (!user?.role_admin) {
    return (
      <Callout intent="danger" title="Access Denied" style={{ borderRadius: 0, margin: "1rem" }}>
        You must be an administrator to view this page.
      </Callout>
    );
  }

  if (isNaN(datasetId)) {
    return (
      <NonIdealState
        icon="warning-sign"
        title="Invalid Dataset ID"
        description="The dataset ID provided in the URL is invalid."
        action={<Button onClick={() => navigate("/spa/datasets")}>Go to Datasets</Button>}
      />
    );
  }

  if (isDatasetLoading) {
    return (
      <div style={{ display: "flex", justifyContent: "center", alignItems: "center", minHeight: "60vh" }}>
        <Spinner size={50} intent="primary" />
      </div>
    );
  }

  if (isDatasetError || !dataset) {
    return (
      <NonIdealState
        icon="error"
        title="Failed to Load Dataset"
        description="Could not load the dataset details. It may have been deleted or the server is unavailable."
        action={
          <div style={{ display: "flex", gap: "0.5rem" }}>
            <Button icon="refresh" onClick={() => refetchDataset()}>Retry</Button>
            <Button onClick={() => navigate("/spa/datasets")}>Back to List</Button>
          </div>
        }
      />
    );
  }

  // Handle Delete
  const handleDeleteDataset = () => {
    if (window.confirm(`Are you sure you want to delete the dataset "${dataset.name}"? This action cannot be undone.`)) {
      deleteMutation.mutate(datasetId, {
        onSuccess: () => {
          navigate("/spa/datasets");
        },
      });
    }
  };

  // Handle Add Books
  const handleToggleSelectBook = (bookId: number) => {
    setSelectedBookIds((prev) => {
      const next = new Set(prev);
      if (next.has(bookId)) {
        next.delete(bookId);
      } else {
        next.add(bookId);
      }
      return next;
    });
  };

  const handleAddSelectedBooks = () => {
    if (selectedBookIds.size === 0) return;
    addBooksMutation.mutate(Array.from(selectedBookIds), {
      onSuccess: () => {
        setSelectedBookIds(new Set());
        refetchDataset();
      },
    });
  };

  // Handle Remove Book
  const handleRemoveBook = (bookId: number) => {
    removeBookMutation.mutate(bookId, {
      onSuccess: () => {
        refetchDataset();
      },
    });
  };

  const handleMemberPageChange = (page: number) => {
    setMemberOffset((page - 1) * memberLimit);
  };

  // Handle Export
  const handleExport = (e: React.FormEvent) => {
    e.preventDefault();
    if (!exportPath.trim()) return;

    exportMutation.mutate(
      { path: exportPath.trim() },
      {
        onSuccess: (data) => {
          updateActiveTaskId(data.task_id);
          queryClient.invalidateQueries({ queryKey: ["tasks-status"] });
        },
        onError: (err: any) => {
          const errMsg = err?.body?.error || err?.message || "Export failed.";
          alert("Export Error: " + errMsg);
        },
      }
    );
  };

  // Active task details
  const activeTask = activeTaskId && tasks ? tasks.find((t) => t.task_id === activeTaskId) : null;
  const isTaskRunning = activeTask && (activeTask.status === "Started" || activeTask.status === "Waiting");

  const getProgressVal = (progStr: any) => {
    if (!progStr || typeof progStr !== "string") return 0;
    const parsed = parseInt(progStr.replace(/[^0-9]/g, ""), 10);
    return isNaN(parsed) ? 0 : parsed / 100;
  };

  // Member books pagination calculations
  const memberTotal = memberBooksData?.total ?? 0;
  const memberTotalPages = Math.ceil(memberTotal / memberLimit);
  const memberCurrentPage = Math.floor(memberOffset / memberLimit) + 1;
  const memberStartIdx = memberOffset + 1;
  const memberEndIdx = Math.min(memberOffset + memberLimit, memberTotal);

  // Available books pagination calculations
  const availableTotal = availableBooksData?.total ?? 0;
  const availableTotalPages = Math.ceil(availableTotal / availableLimit);
  const availableCurrentPage = Math.floor(availableOffset / availableLimit) + 1;
  const availableStartIdx = availableOffset + 1;
  const availableEndIdx = Math.min(availableOffset + availableLimit, availableTotal);

  return (
    <div style={{ padding: "1.5rem", display: "flex", flexDirection: "column", gap: "1.5rem" }}>
      {/* Navigation */}
      <div>
        <Button
          icon="chevron-left"
          variant="minimal"
          onClick={() => navigate("/spa/datasets")}
          style={{ fontFamily: "Space Mono, monospace", textTransform: "uppercase", paddingLeft: 0 }}
        >
          Back to Datasets
        </Button>
      </div>

      {/* Header section */}
      <Card
        style={{
          borderRadius: 0,
          border: "1px solid var(--border-color)",
          backgroundColor: "var(--bg-secondary)",
          padding: "1.5rem",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          flexWrap: "wrap",
          gap: "1rem",
        }}
      >
        <div style={{ flex: 1, minWidth: "300px" }}>
          <H2 style={{ fontFamily: "Playfair Display, serif", margin: "0 0 0.5rem 0", color: "var(--accent-primary)" }}>
            {dataset.name}
          </H2>
          <p style={{ color: "var(--text-secondary)", margin: 0 }}>
            {dataset.description || "No description provided."}
          </p>
          <div style={{ display: "flex", gap: "1.5rem", marginTop: "1rem", fontSize: "0.8rem", color: "var(--text-muted)", fontFamily: "Space Mono, monospace" }}>
            <span>UUID: {dataset.uuid}</span>
            <span>Books: {dataset.book_count}</span>
            <span>Created: {new Date(dataset.created).toLocaleString()}</span>
          </div>
        </div>
        <div style={{ display: "flex", gap: "0.5rem" }}>
          <Button
            icon="edit"
            onClick={() => setIsEditDialogOpen(true)}
            style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
          >
            Edit Dataset
          </Button>
          <Button
            icon="trash"
            intent="danger"
            onClick={handleDeleteDataset}
            loading={deleteMutation.isPending}
            style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
          >
            Delete
          </Button>
        </div>
      </Card>

      {/* Two Column details */}
      <div style={{ display: "flex", gap: "1.5rem", flexWrap: "wrap" }}>
        {/* Left Column: Metadata & Export */}
        <div style={{ flex: "1 1 350px", display: "flex", flexDirection: "column", gap: "1.5rem" }}>
          {/* Metadata Card */}
          <Card style={{ borderRadius: 0, border: "1px solid var(--border-color)", backgroundColor: "var(--bg-secondary)" }}>
            <H5 style={{ borderBottom: "1px solid var(--border-light)", paddingBottom: "0.5rem", textTransform: "uppercase", fontSize: "0.8rem", letterSpacing: "0.05em" }}>
              Dataset Metadata
            </H5>
            {dataset.metadata.length === 0 ? (
              <p style={{ color: "var(--text-muted)", fontSize: "0.9rem", fontStyle: "italic", margin: "1rem 0 0 0" }}>
                No metadata items defined. Click "Edit Dataset" to add fields like model target, data source, or version keys.
              </p>
            ) : (
              <HTMLTable bordered striped style={{ width: "100%", marginTop: "0.75rem" }}>
                <thead>
                  <tr style={{ fontSize: "0.75rem", textTransform: "uppercase" }}>
                    <th>Key</th>
                    <th>Value</th>
                    <th>Type</th>
                  </tr>
                </thead>
                <tbody>
                  {dataset.metadata.map((item, idx) => (
                    <tr key={idx} style={{ fontSize: "0.85rem" }}>
                      <td style={{ fontFamily: "Space Mono, monospace", fontWeight: "bold" }}>{item.key}</td>
                      <td style={{ wordBreak: "break-all" }}>{item.value}</td>
                      <td>
                        <span
                          style={{
                            fontSize: "0.7rem",
                            textTransform: "uppercase",
                            fontFamily: "Space Mono, monospace",
                            background: "var(--bg-primary)",
                            padding: "2px 4px",
                            border: "1px solid var(--border-light)",
                          }}
                        >
                          {item.value_type}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </HTMLTable>
            )}
          </Card>

          {/* Export Card */}
          <Card style={{ borderRadius: 0, border: "1px solid var(--border-color)", backgroundColor: "var(--bg-secondary)" }}>
            <H5 style={{ borderBottom: "1px solid var(--border-light)", paddingBottom: "0.5rem", textTransform: "uppercase", fontSize: "0.8rem", letterSpacing: "0.05em" }}>
              Export to Markdown
            </H5>
            <p style={{ color: "var(--text-secondary)", fontSize: "0.85rem", margin: "0.5rem 0 1rem 0" }}>
              Iterates through the books in the dataset and converts their contents to Markdown files. Skipping image-based PDFs.
            </p>

            <form onSubmit={handleExport} style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
              <InputGroup
                placeholder="/absolute/path/to/export/directory"
                value={exportPath}
                onChange={(e) => setExportPath(e.target.value)}
                disabled={isTaskRunning || exportMutation.isPending}
                style={{ borderRadius: 0 }}
                required
              />
              <Button
                type="submit"
                intent="primary"
                loading={exportMutation.isPending}
                disabled={isTaskRunning || !exportPath.trim()}
                style={{ borderRadius: 0, fontFamily: "Space Mono, monospace", textTransform: "uppercase" }}
              >
                Trigger Export
              </Button>
            </form>

            {/* Task Tracking UI */}
            {activeTaskId && (
              <div style={{ marginTop: "1.25rem", borderTop: "1px dashed var(--border-light)", paddingTop: "1rem" }}>
                <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.8rem", fontWeight: "bold", marginBottom: "0.5rem", alignItems: "center" }}>
                  <span>Task ID: {activeTaskId}</span>
                  <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
                    <span style={{ color: activeTask?.status === "Finished" ? "var(--accent-green)" : activeTask?.status === "Failed" ? "var(--accent-red)" : "inherit" }}>
                      Status: {activeTask?.status || "Starting..."}
                    </span>
                    {!isTaskRunning && (
                      <Button icon="cross" variant="minimal" onClick={() => updateActiveTaskId(null)} style={{ padding: 0, minHeight: 0 }} />
                    )}
                  </div>
                </div>
                {activeTask ? (
                  <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                    <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.85rem" }}>
                      <span>Progress</span>
                      <span>{activeTask.progress}</span>
                    </div>
                    <ProgressBar
                      value={getProgressVal(activeTask.progress)}
                      intent={
                        activeTask.status === "Finished"
                          ? Intent.SUCCESS
                          : activeTask.status === "Failed"
                          ? Intent.DANGER
                          : Intent.PRIMARY
                      }
                      stripes={!!isTaskRunning}
                    />
                    {activeTask.taskMessage && (
                      <div style={{ fontSize: "0.85rem", color: "var(--text-secondary)", marginTop: "0.25rem", fontStyle: "italic", wordBreak: "break-all" }}>
                        {activeTask.taskMessage}
                      </div>
                    )}
                    {activeTask.error && (
                      <Callout intent="danger" title="Task Error" style={{ borderRadius: 0, fontSize: "0.8rem", marginTop: "0.5rem" }}>
                        {activeTask.error}
                      </Callout>
                    )}
                    {isTaskRunning && (
                      <Button
                        icon="stop"
                        intent="danger"
                        variant="minimal"
                        onClick={() => cancelTaskMutation.mutate(activeTaskId)}
                        loading={cancelTaskMutation.isPending}
                        style={{ alignSelf: "flex-end", marginTop: "0.25rem", fontSize: "0.8rem" }}
                      >
                        Cancel Task
                      </Button>
                    )}
                  </div>
                ) : (
                  <div style={{ fontSize: "0.85rem", color: "var(--text-secondary)", fontStyle: "italic" }}>
                    Waiting for task info to load from server...
                  </div>
                )}
              </div>
            )}

            {/* Background Task Queue Failsafe */}
            {tasks && tasks.length > 0 && (
              <div style={{ marginTop: "1.25rem", borderTop: "1px dashed var(--border-light)", paddingTop: "1rem" }}>
                <span style={{ fontSize: "0.85rem", fontWeight: "bold", display: "block", marginBottom: "0.5rem" }}>Background Task Queue</span>
                <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                  {tasks.map((t) => (
                    <div key={t.task_id} style={{ display: "flex", flexDirection: "column", gap: "0.25rem", padding: "0.5rem", border: "1px solid var(--border-light)", backgroundColor: "var(--bg-primary)" }}>
                      <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.8rem", fontWeight: "bold" }}>
                        <span style={{ wordBreak: "break-all" }}>{t.taskMessage}</span>
                        <span style={{ color: t.status === "Finished" ? "var(--accent-green)" : t.status === "Failed" ? "var(--accent-red)" : "inherit" }}>{t.status}</span>
                      </div>
                      <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.75rem", color: "var(--text-secondary)" }}>
                        <span>ID: {t.task_id}</span>
                        <span>{t.progress}</span>
                      </div>
                      <ProgressBar
                        value={getProgressVal(t.progress)}
                        intent={
                          t.status === "Finished"
                            ? Intent.SUCCESS
                            : t.status === "Failed"
                            ? Intent.DANGER
                            : Intent.PRIMARY
                        }
                        stripes={t.status === "Started" || t.status === "Waiting"}
                      />
                    </div>
                  ))}
                </div>
              </div>
            )}
          </Card>
        </div>

        {/* Right Column: Books in Dataset */}
        <div style={{ flex: "2 2 500px", display: "flex", flexDirection: "column", gap: "1.5rem" }}>
          {/* Books List Card */}
          <Card style={{ borderRadius: 0, border: "1px solid var(--border-color)", backgroundColor: "var(--bg-secondary)", display: "flex", flexDirection: "column", gap: "1rem" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid var(--border-light)", paddingBottom: "0.5rem" }}>
              <H3 style={{ margin: 0, fontSize: "1.1rem" }}>Books in Dataset</H3>
              <Button
                icon="plus"
                intent="success"
                onClick={() => setShowAddBooks(true)}
                style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
              >
                Add Books
              </Button>
            </div>

            {/* Books inside the dataset */}
            {isMemberBooksLoading ? (
              <div style={{ display: "flex", justifyContent: "center", padding: "2rem" }}>
                <Spinner size={30} />
              </div>
            ) : !memberBooksData || memberBooksData.rows.length === 0 ? (
              <p style={{ color: "var(--text-muted)", fontSize: "0.9rem", fontStyle: "italic", margin: "1rem 0" }}>
                This dataset has no books yet. Click "Add Books" above to browse and insert books.
              </p>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
                {memberTotalPages > 1 && (
                  <Pagination
                    currentPage={memberCurrentPage}
                    totalPages={memberTotalPages}
                    total={memberTotal}
                    startIdx={memberStartIdx}
                    endIdx={memberEndIdx}
                    onPageChange={handleMemberPageChange}
                  />
                )}
                <div style={{ display: "grid", gridTemplateColumns: "1fr", gap: "1rem" }}>
                  {memberBooksData.rows.map((book, idx) => (
                    <RowCard
                      key={book.id}
                      book={book}
                      index={memberOffset + idx + 1}
                      onClick={() => {}}
                      onRemove={() => handleRemoveBook(book.id)}
                    />
                  ))}
                </div>
              </div>
            )}
          </Card>

            {/* Add Books Dialog Popup */}
            <Dialog
              isOpen={showAddBooks}
              onClose={() => {
                setShowAddBooks(false);
                setSelectedBookIds(new Set());
              }}
              title="Add Books to Dataset"
              style={{
                borderRadius: 0,
                backgroundColor: "var(--bg-secondary)",
                width: "90%",
                maxWidth: "1000px",
                paddingBottom: 0,
              }}
            >
              <div style={{ padding: "1.5rem", display: "flex", flexDirection: "column", gap: "1rem" }}>
                {/* Header Row */}
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid var(--border-light)", paddingBottom: "0.75rem" }}>
                  <div style={{ display: "flex", gap: "1rem", alignItems: "center" }}>
                    <div style={{ width: "300px" }}>
                      <InputGroup
                        leftIcon="search"
                        placeholder="Filter available books..."
                        value={availableSearch}
                        onChange={(e) => {
                          setAvailableSearch(e.target.value);
                          setAvailableOffset(0);
                        }}
                        style={{ borderRadius: 0 }}
                      />
                    </div>
                    <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
                      <span style={{ fontSize: "0.85rem", color: "var(--text-secondary)" }}>Page Size:</span>
                      <HTMLSelect
                        value={availableLimit}
                        onChange={(e) => {
                          setAvailableLimit(parseInt(e.target.value, 10));
                          setAvailableOffset(0);
                        }}
                        options={[
                          { label: "5 per page", value: 5 },
                          { label: "10 per page", value: 10 },
                          { label: "20 per page", value: 20 },
                          { label: "50 per page", value: 50 },
                          { label: "100 per page", value: 100 },
                        ]}
                        style={{ borderRadius: 0 }}
                      />
                    </div>
                  </div>
                  
                  <div style={{ display: "flex", gap: "1rem", alignItems: "center" }}>
                    <span style={{ fontSize: "0.85rem", color: "var(--text-secondary)", fontFamily: "Space Mono, monospace" }}>
                      Selected: {selectedBookIds.size}
                    </span>
                    <Button
                      intent="success"
                      disabled={selectedBookIds.size === 0}
                      loading={addBooksMutation.isPending}
                      onClick={handleAddSelectedBooks}
                      style={{ borderRadius: 0, fontFamily: "Space Mono, monospace", textTransform: "uppercase" }}
                    >
                      Add Selected
                    </Button>
                    <Button
                      onClick={() => {
                        setShowAddBooks(false);
                        setSelectedBookIds(new Set());
                      }}
                      style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
                    >
                      Close
                    </Button>
                  </div>
                </div>
  
                {/* Grid Content */}
                {isAvailableBooksLoading ? (
                  <div style={{ display: "flex", justifyContent: "center", padding: "4rem" }}>
                    <Spinner size={40} />
                  </div>
                ) : !availableBooksData || availableBooksData.rows.length === 0 ? (
                  <p style={{ color: "var(--text-muted)", fontSize: "0.9rem", fontStyle: "italic", textAlign: "center", padding: "2rem" }}>
                    No available books found matching the search criteria.
                  </p>
                ) : (
                  <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
                    {availableTotalPages > 1 && (
                      <Pagination
                        currentPage={availableCurrentPage}
                        totalPages={availableTotalPages}
                        total={availableTotal}
                        startIdx={availableStartIdx}
                        endIdx={availableEndIdx}
                        onPageChange={(page) => setAvailableOffset((page - 1) * availableLimit)}
                      />
                    )}
                    <div
                      style={{
                        display: "grid",
                        gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))",
                        gap: "1rem",
                        maxHeight: "55vh",
                        overflowY: "auto",
                        paddingRight: "0.25rem",
                      }}
                    >
                      {availableBooksData.rows.map((book, idx) => (
                        <RowCard
                          key={book.id}
                          book={book}
                          index={availableOffset + idx + 1}
                          onClick={() => handleToggleSelectBook(book.id)}
                          selectable={true}
                          selected={selectedBookIds.has(book.id)}
                          onToggleSelect={() => handleToggleSelectBook(book.id)}
                        />
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </Dialog>
          </div>
        </div>
  
        {/* Edit Form Dialog */}
        <DatasetFormDialog
          isOpen={isEditDialogOpen}
          mode="edit"
          dataset={dataset}
          onClose={() => setIsEditDialogOpen(false)}
          onSuccess={() => refetchDataset()}
        />
      </div>
    );
  }
