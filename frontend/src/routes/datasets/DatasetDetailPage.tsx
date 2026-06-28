import { useState, useEffect } from "react";
import { useParams, useNavigate, useSearchParams } from "react-router-dom";
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
  DialogBody,
  DialogFooter,
  HTMLSelect,
  OverlayToaster,
  Position,
  Collapse,
  Tabs,
  Tab,
  NumericInput,
  ButtonGroup,
  Checkbox,
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
  useExportChunkedDataset,
  useTasksStatus,
  useCancelTask,
} from "../../hooks/useDatasets";
import { RowCard } from "../../components/RowCard";
import { Pagination } from "../../components/Pagination";
import { DatasetFormDialog } from "../../components/DatasetFormDialog";
import { BookDetailDialog } from "../../components/BookDetailDialog";

const AppToaster = OverlayToaster.createAsync({
  position: Position.TOP,
});

export default function DatasetDetailPage() {
  const { id: idParam } = useParams<{ id: string }>();
  const datasetId = parseInt(idParam || "", 10);
  const navigate = useNavigate();
  const { user } = useApp();
  const queryClient = useQueryClient();

  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);

  // Export Dialog state
  const [isExportDialogOpen, setIsExportDialogOpen] = useState(false);
  const [exportTab, setExportTab] = useState<"plain" | "chunked">("plain");
  const [exportPath, setExportPath] = useState("");
  const [chunkPath, setChunkPath] = useState("");
  const [chunkSize, setChunkSize] = useState<number>(768);
  const [chunkOverlap, setChunkOverlap] = useState<number>(80);
  const [forceExport, setForceExport] = useState(false);

  // Delete Dialog state
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [deleteConfirmInput, setDeleteConfirmInput] = useState("");

  // Remove Book Dialog state
  const [pendingRemoveBook, setPendingRemoveBook] = useState<{ id: number; title: string } | null>(null);

  // Metadata expand state
  const [expanded, setExpanded] = useState(false);

  const oldKey = `qalibre_active_export_task_${datasetId}`;
  const newKey = `qalibre_active_dataset_task_${datasetId}`;
  const [activeTaskId, setActiveTaskId] = useState<string | null>(() => {
    const oldVal = localStorage.getItem(oldKey);
    if (oldVal) {
      localStorage.setItem(newKey, oldVal);
      localStorage.removeItem(oldKey);
      return oldVal;
    }
    return localStorage.getItem(newKey) || null;
  });

  const updateActiveTaskId = (id: string | null) => {
    setActiveTaskId(id);
    if (id) {
      localStorage.setItem(newKey, id);
    } else {
      localStorage.removeItem(newKey);
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
  const [selectedBookId, setSelectedBookId] = useState<number | null>(null);

  const [searchParams] = useSearchParams();
  const cover = searchParams.get("cover") || "md";
  const gridMin = cover === "lg" ? 360 : 320;

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
  const exportChunkedMutation = useExportChunkedDataset(datasetId);
  const cancelTaskMutation = useCancelTask();

  // Prefill path defaults when dataset details load
  useEffect(() => {
    if (dataset?.export_directory) {
      setExportPath(dataset.export_directory);
      setChunkPath(dataset.export_directory);
    }
  }, [dataset]);

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

  // Handle Delete Confirmation Submit
  const handleConfirmDelete = () => {
    if (deleteConfirmInput.trim() !== dataset.name) return;
    deleteMutation.mutate(datasetId, {
      onSuccess: () => {
        setIsDeleteDialogOpen(false);
        navigate("/spa/datasets");
      },
    });
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
        AppToaster.then((toaster) => {
          toaster.show({
            message: "Operation succeeded: Books added to dataset",
            intent: Intent.SUCCESS,
            icon: "tick",
            timeout: 2000,
          });
        });
      },
      onError: (err: any) => {
        const errMsg = err?.body?.error || err?.message || "Failed to add books.";
        AppToaster.then((toaster) => {
          toaster.show({
            message: `Error: ${errMsg}`,
            intent: Intent.DANGER,
            icon: "error",
            timeout: 3000,
          });
        });
      },
    });
  };

  // Handle Remove Book
  const handleRemoveBook = (bookId: number) => {
    removeBookMutation.mutate(bookId, {
      onSuccess: () => {
        refetchDataset();
        AppToaster.then((toaster) => {
          toaster.show({
            message: "Operation succeeded: Book removed from dataset",
            intent: Intent.SUCCESS,
            icon: "tick",
            timeout: 2000,
          });
        });
      },
      onError: (err: any) => {
        const errMsg = err?.body?.error || err?.message || "Failed to remove book.";
        AppToaster.then((toaster) => {
          toaster.show({
            message: `Error: ${errMsg}`,
            intent: Intent.DANGER,
            icon: "error",
            timeout: 3000,
          });
        });
      },
    });
  };

  const handleMemberPageChange = (page: number) => {
    setMemberOffset((page - 1) * memberLimit);
  };

  // Handle Export Submit
  const handleExportSubmit = () => {
    if (exportTab === "plain") {
      if (!exportPath.trim()) return;
      exportMutation.mutate(
        {
          path: exportPath.trim(),
          force: forceExport,
        },
        {
          onSuccess: (data) => {
            updateActiveTaskId(data.task_id);
            setIsExportDialogOpen(false);
            queryClient.invalidateQueries({ queryKey: ["tasks-status"] });
          },
          onError: (err: any) => {
            const errMsg = err?.body?.error || err?.message || "Export failed.";
            AppToaster.then((toaster) => {
              toaster.show({
                message: `Export Error: ${errMsg}`,
                intent: Intent.DANGER,
                icon: "error",
                timeout: 3000,
              });
            });
          },
        }
      );
    } else {
      if (chunkOverlap >= chunkSize) {
        AppToaster.then((toaster) => {
          toaster.show({
            message: "Chunk overlap must be less than chunk size.",
            intent: Intent.WARNING,
            timeout: 3000,
          });
        });
        return;
      }
      exportChunkedMutation.mutate(
        {
          path: chunkPath.trim() || undefined,
          chunk_size: chunkSize,
          chunk_overlap: chunkOverlap,
          force: forceExport,
        },
        {
          onSuccess: (data) => {
            updateActiveTaskId(data.task_id);
            setIsExportDialogOpen(false);
            queryClient.invalidateQueries({ queryKey: ["tasks-status"] });
          },
          onError: (err: any) => {
            const errMsg = err?.body?.error || err?.message || "Chunked export failed.";
            AppToaster.then((toaster) => {
              toaster.show({
                message: `Export Error: ${errMsg}`,
                intent: Intent.DANGER,
                icon: "error",
                timeout: 3000,
              });
            });
          },
        }
      );
    }
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

      {/* Header section (restructured header card) */}
      <Card
        style={{
          borderRadius: 0,
          border: "1px solid var(--border-color)",
          backgroundColor: "var(--bg-secondary)",
          padding: "1.5rem",
          display: "flex",
          flexDirection: "column",
          gap: "1rem",
        }}
      >
        <div>
          <H2 style={{ fontFamily: "Playfair Display, serif", margin: "0 0 0.5rem 0", color: "var(--accent-primary)" }}>
            {dataset.name}
          </H2>
          {dataset.description && (
            <p style={{ color: "var(--text-secondary)", margin: "0 0 0.75rem 0" }}>
              {dataset.description}
            </p>
          )}
          <div style={{ display: "flex", gap: "1.5rem", fontSize: "0.8rem", color: "var(--text-muted)", fontFamily: "Space Mono, monospace" }}>
            <span>UUID: {dataset.uuid}</span>
            <span>Books count: {dataset.book_count}</span>
            <span>Created: {new Date(dataset.created).toLocaleString()}</span>
          </div>
        </div>

        {/* Expand Metadata & Parameters button */}
        <div>
          <Button
            icon={expanded ? "chevron-up" : "chevron-down"}
            variant="minimal"
            onClick={() => setExpanded(!expanded)}
            style={{ fontFamily: "Space Mono, monospace", padding: 0 }}
          >
            {expanded ? "Collapse" : "Expand Metadata & Parameters"}
          </Button>
        </div>

        {/* Expandable Section */}
        <Collapse isOpen={expanded}>
          <div style={{ display: "flex", flexDirection: "column", gap: "1.5rem", marginTop: "0.5rem", borderTop: "1px dashed var(--border-light)", paddingTop: "1rem" }}>
            {/* Metadata table */}
            <div>
              <H5 style={{ textTransform: "uppercase", fontSize: "0.75rem", letterSpacing: "0.05em", margin: "0 0 0.5rem 0" }}>
                Dataset Metadata
              </H5>
              {dataset.metadata.length === 0 ? (
                <p style={{ color: "var(--text-muted)", fontSize: "0.85rem", fontStyle: "italic", margin: 0 }}>
                  No metadata items defined.
                </p>
              ) : (
                <HTMLTable bordered striped style={{ width: "100%" }}>
                  <thead>
                    <tr style={{ fontSize: "0.7rem", textTransform: "uppercase" }}>
                      <th>Key</th>
                      <th>Value</th>
                      <th>Type</th>
                    </tr>
                  </thead>
                  <tbody>
                    {dataset.metadata.map((item, idx) => (
                      <tr key={idx} style={{ fontSize: "0.8rem" }}>
                        <td style={{ fontFamily: "Space Mono, monospace", fontWeight: "bold" }}>{item.key}</td>
                        <td style={{ wordBreak: "break-all" }}>{item.value}</td>
                        <td>
                          <span
                            style={{
                              fontSize: "0.65rem",
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
            </div>

            {/* Export directory */}
            <div>
              <H5 style={{ textTransform: "uppercase", fontSize: "0.75rem", letterSpacing: "0.05em", margin: "0 0 0.5rem 0" }}>
                Export Directory
              </H5>
              {dataset.export_directory ? (
                <Callout intent="primary" style={{ borderRadius: 0, padding: "0.75rem" }}>
                  <InputGroup readOnly value={dataset.export_directory} leftIcon="folder-close" style={{ fontFamily: "sans-serif" }} />
                </Callout>
              ) : (
                <Callout intent="warning" icon="warning-sign" title="No export directory configured" style={{ borderRadius: 0 }}>
                  Set an export directory in Edit Dataset, otherwise exports will need an explicit path each time.
                </Callout>
              )}
            </div>

            {/* Default parameters */}
            <div>
              <H5 style={{ textTransform: "uppercase", fontSize: "0.75rem", letterSpacing: "0.05em", margin: "0 0 0.5rem 0" }}>
                Chunking Parameters (defaults used if not overridden per-export)
              </H5>
              <HTMLTable bordered striped style={{ width: "100%" }}>
                <thead>
                  <tr style={{ fontSize: "0.7rem", textTransform: "uppercase" }}>
                    <th>Parameter</th>
                    <th>Default</th>
                    <th>Description</th>
                  </tr>
                </thead>
                <tbody>
                  <tr style={{ fontSize: "0.8rem" }}>
                    <td style={{ fontFamily: "Space Mono, monospace" }}>chunk_size</td>
                    <td>768</td>
                    <td>Maximum characters per chunk, passed to <code>langchain.text_splitter.MarkdownTextSplitter</code>.</td>
                  </tr>
                  <tr style={{ fontSize: "0.8rem" }}>
                    <td style={{ fontFamily: "Space Mono, monospace" }}>chunk_overlap</td>
                    <td>80</td>
                    <td>Overlap characters between consecutive chunks.</td>
                  </tr>
                  <tr style={{ fontSize: "0.8rem" }}>
                    <td style={{ fontFamily: "Space Mono, monospace" }}>Engine</td>
                    <td>langchain</td>
                    <td>Chunking is performed by LangChain's <code>MarkdownTextSplitter</code>. The markdown text is produced by <code>pymupdf4llm.to_markdown</code> first.</td>
                  </tr>
                </tbody>
              </HTMLTable>
            </div>
          </div>
        </Collapse>

        {/* Action Button Group */}
        <div style={{ display: "flex", justifyContent: "flex-end", marginTop: "0.5rem" }}>
          <ButtonGroup>
            <Button
              icon="edit"
              onClick={() => setIsEditDialogOpen(true)}
              style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
            >
              Edit
            </Button>
            <Button
              icon="export"
              rightIcon="caret-down"
              onClick={() => {
                // reset dialog defaults
                if (dataset?.export_directory) {
                  setExportPath(dataset.export_directory);
                  setChunkPath(dataset.export_directory);
                }
                setIsExportDialogOpen(true);
              }}
              style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
            >
              Export
            </Button>
            <Button
              icon="trash"
              intent="danger"
              onClick={() => {
                setDeleteConfirmInput("");
                setIsDeleteDialogOpen(true);
              }}
              loading={deleteMutation.isPending}
              style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
            >
              Delete
            </Button>
          </ButtonGroup>
        </div>
      </Card>

      {/* Main Single Primary Column Area */}
      <Card
        style={{
          borderRadius: 0,
          border: "1px solid var(--border-color)",
          backgroundColor: "var(--bg-secondary)",
          display: "flex",
          flexDirection: "column",
          gap: "1rem",
        }}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid var(--border-light)", paddingBottom: "0.5rem" }}>
          <H3 style={{ margin: 0, fontSize: "1.1rem" }}>Books in Dataset ({memberTotal})</H3>
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
            <div
              className="book-grid"
              style={{
                display: "grid",
                gridTemplateColumns: `repeat(auto-fill, minmax(${gridMin}px, 1fr))`,
                gap: "1.5rem",
                alignContent: "start",
                flex: 1,
                padding: "1.5rem 0",
              }}
            >
              {memberBooksData.rows.map((book, idx) => (
                <RowCard
                  key={book.id}
                  book={book}
                  index={memberOffset + idx + 1}
                  onClick={() => setSelectedBookId(book.id)}
                  onRemove={() => setPendingRemoveBook({ id: book.id, title: book.title })}
                />
              ))}
            </div>
          </div>
        )}
      </Card>

      {/* Task Tracking UI - Moved below book list */}
      {activeTaskId && (
        <Card style={{ borderRadius: 0, border: "1px solid var(--border-color)", backgroundColor: "var(--bg-secondary)" }}>
          <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.8rem", fontWeight: "bold", marginBottom: "0.5rem", alignItems: "center" }}>
            <span>Active Export Task: {activeTaskId}</span>
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
        </Card>
      )}

      {/* Background Task Queue */}
      {tasks && tasks.length > 0 && (
        <Card style={{ borderRadius: 0, border: "1px solid var(--border-color)", backgroundColor: "var(--bg-secondary)" }}>
          <span style={{ fontSize: "0.85rem", fontWeight: "bold", display: "block", marginBottom: "0.5rem" }}>Background Task Queue</span>
          <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
            {tasks.map((t) => (
              <div key={t.task_id} style={{ display: "flex", flexDirection: "column", gap: "0.25rem", padding: "0.5rem", border: "1px solid var(--border-light)", backgroundColor: "var(--bg-primary)" }}>
                <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.8rem", fontWeight: "bold" }}>
                  <span style={{ wordBreak: "break-all" }}>{t.taskMessage}</span>
                  <span style={{ color: t.status === "Finished" ? "var(--accent-green)" : t.status === "Failed" ? "var(--accent-red)" : "inherit" }}>{t.status}</span>
                </div>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: "0.75rem", color: "var(--text-secondary)" }}>
                  <span>ID: {t.task_id}</span>
                  <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                    <span>{t.progress}</span>
                    {(t.status === "Started" || t.status === "Waiting") && (
                      <Button
                        icon="stop"
                        intent="danger"
                        variant="minimal"
                        small
                        onClick={() => cancelTaskMutation.mutate(t.task_id)}
                        loading={cancelTaskMutation.isPending && cancelTaskMutation.variables === t.task_id}
                        style={{ padding: "0 2px", minHeight: 20, height: 20 }}
                      />
                    )}
                  </div>
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
        </Card>
      )}

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

      {/* Edit Form Dialog */}
      <DatasetFormDialog
        isOpen={isEditDialogOpen}
        mode="edit"
        dataset={dataset}
        onClose={() => setIsEditDialogOpen(false)}
        onSuccess={() => refetchDataset()}
      />

      {/* Export Dialog (New) */}
      <Dialog
        isOpen={isExportDialogOpen}
        onClose={() => {
          setIsExportDialogOpen(false);
          setForceExport(false);
        }}
        title="Export Dataset"
        icon="export"
        style={{ borderRadius: 0, backgroundColor: "var(--bg-secondary)", width: 640, maxWidth: "95vw" }}
      >
        <DialogBody>
          <Tabs
            id="export-tabs"
            selectedTabId={exportTab}
            onChange={(id) => setExportTab(id as "plain" | "chunked")}
            renderActiveTabPanelOnly
          >
            <Tab
              id="plain"
              title="Export Markdown"
              panel={
                <div style={{ display: "flex", flexDirection: "column", gap: "1rem", marginTop: "1rem" }}>
                  <Callout intent="primary" style={{ borderRadius: 0 }}>
                    Plain Markdown files will be written to <code>{`<destination_directory>/${dataset.name}/`}</code>. Existing files with the same name are skipped.
                  </Callout>
                  <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                    <label style={{ fontSize: "0.75rem", fontWeight: "bold", textTransform: "uppercase" }}>Destination Directory (Absolute path)</label>
                    <InputGroup
                      leftIcon="folder-close"
                      placeholder="/absolute/path/to/root"
                      value={exportPath}
                      onChange={(e) => setExportPath(e.target.value)}
                      style={{ borderRadius: 0 }}
                    />
                  </div>
                </div>
              }
            />
            <Tab
              id="chunked"
              title="Export Chunked Markdown"
              panel={
                <div style={{ display: "flex", flexDirection: "column", gap: "1rem", marginTop: "1rem" }}>
                  <Callout intent="primary" title="Chunking via LangChain" style={{ borderRadius: 0 }}>
                    Chunked Markdown export splits each book's Markdown text into smaller overlapping chunks suitable for embeddings and RAG ingestion. The Markdown text is generated by <code>pymupdf4llm.to_markdown</code> first, then chunked by <code>langchain.text_splitter.MarkdownTextSplitter</code>. Chunk files are written to <code>{`<path>/${dataset.name}/chunked/`}</code> and named <code>{`<title>__chunk_NNN.md`}</code>. Each chunk file includes the dataset metadata as YAML front matter.
                  </Callout>
                  
                  <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                    <label style={{ fontSize: "0.75rem", fontWeight: "bold", textTransform: "uppercase" }}>Destination Directory (Absolute path, optional - defaults to dataset's export directory)</label>
                    <InputGroup
                      leftIcon="folder-close"
                      placeholder={dataset.export_directory || "/absolute/path/to/root"}
                      value={chunkPath}
                      onChange={(e) => setChunkPath(e.target.value)}
                      style={{ borderRadius: 0 }}
                    />
                  </div>

                  <div style={{ display: "flex", gap: "1rem" }}>
                    <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                      <label style={{ fontSize: "0.75rem", fontWeight: "bold", textTransform: "uppercase" }}>Chunk Size (characters, default 768)</label>
                      <NumericInput
                        min={50}
                        max={100000}
                        stepSize={50}
                        majorStepSize={500}
                        minorStepSize={10}
                        value={chunkSize.toString()}
                        onValueChange={(_, s) => setChunkSize(parseInt(s || "768", 10))}
                        fill
                        style={{ borderRadius: 0 }}
                      />
                    </div>
                    <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                      <label style={{ fontSize: "0.75rem", fontWeight: "bold", textTransform: "uppercase" }}>Chunk Overlap (characters, default 80, must be &lt; chunk size)</label>
                      <NumericInput
                        min={0}
                        max={99999}
                        stepSize={10}
                        majorStepSize={50}
                        minorStepSize={1}
                        value={chunkOverlap.toString()}
                        onValueChange={(_, s) => setChunkOverlap(parseInt(s || "80", 10))}
                        fill
                        style={{ borderRadius: 0 }}
                      />
                    </div>
                  </div>
                </div>
              }
            />
          </Tabs>
          <div style={{ marginTop: "1.5rem", borderTop: "1px solid var(--border-color)", paddingTop: "1rem" }}>
            <Checkbox
              checked={forceExport}
              onChange={(e) => setForceExport((e.target as HTMLInputElement).checked)}
              label="Force re-export (ignore idempotency and overwrite existing files)"
              style={{ marginBottom: 0 }}
            />
          </div>
        </DialogBody>
        <DialogFooter
          actions={
            <>
              <Button onClick={() => setIsExportDialogOpen(false)} style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}>Cancel</Button>
              <Button
                intent="primary"
                loading={exportMutation.isPending || exportChunkedMutation.isPending}
                onClick={handleExportSubmit}
                disabled={exportTab === "plain" ? !exportPath.trim() : false}
                style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
              >
                {exportTab === "plain" ? "Start Export" : "Start Chunked Export"}
              </Button>
            </>
          }
        />
      </Dialog>

      {/* Delete Confirmation Dialog (New) */}
      <Dialog
        isOpen={isDeleteDialogOpen}
        onClose={() => setIsDeleteDialogOpen(false)}
        title="Delete Dataset"
        icon="trash"
        style={{ borderRadius: 0, backgroundColor: "var(--bg-secondary)", width: 520, maxWidth: "95vw" }}
      >
        <DialogBody>
          <Callout intent="danger" title="This action cannot be undone" style={{ borderRadius: 0, marginBottom: "1rem" }}>
            Deleting "<strong>{dataset.name}</strong>" permanently removes the dataset, all its metadata, and the membership links to its {dataset.book_count} book(s). The books themselves are NOT deleted from calibre.
          </Callout>
          <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
            <label style={{ fontSize: "0.75rem", fontWeight: "bold" }}>
              Type the dataset name <code>{dataset.name}</code> to confirm
            </label>
            <InputGroup
              placeholder={dataset.name}
              value={deleteConfirmInput}
              onChange={(e) => setDeleteConfirmInput(e.target.value)}
              autoFocus
              intent={deleteConfirmInput && deleteConfirmInput !== dataset.name ? "danger" : "none"}
              style={{ borderRadius: 0 }}
            />
          </div>
        </DialogBody>
        <DialogFooter
          actions={
            <>
              <Button onClick={() => setIsDeleteDialogOpen(false)} style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}>Cancel</Button>
              <Button
                intent="danger"
                disabled={deleteConfirmInput.trim() !== dataset.name}
                loading={deleteMutation.isPending}
                onClick={handleConfirmDelete}
                style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
              >
                Delete Dataset
              </Button>
            </>
          }
        />
      </Dialog>

      {/* Remove Book Confirmation Dialog (New) */}
      <Dialog
        isOpen={!!pendingRemoveBook}
        onClose={() => setPendingRemoveBook(null)}
        title="Remove Book from Dataset"
        icon="cross"
        style={{ width: 460, borderRadius: 0, backgroundColor: "var(--bg-secondary)" }}
      >
        <DialogBody>
          <p>
            Remove <strong>{pendingRemoveBook?.title}</strong> from this dataset? The book will remain in your calibre library.
          </p>
        </DialogBody>
        <DialogFooter
          actions={
            <>
              <Button onClick={() => setPendingRemoveBook(null)} style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}>Cancel</Button>
              <Button
                intent="danger"
                loading={removeBookMutation.isPending}
                onClick={() => {
                  if (!pendingRemoveBook) return;
                  const id = pendingRemoveBook.id;
                  setPendingRemoveBook(null);
                  handleRemoveBook(id);
                }}
                style={{ borderRadius: 0, fontFamily: "Space Mono, monospace" }}
              >
                Remove
              </Button>
            </>
          }
        />
      </Dialog>

      {/* Detail Dialog */}
      {selectedBookId !== null && (
        <BookDetailDialog
          bookId={selectedBookId}
          isOpen={selectedBookId !== null}
          onClose={() => setSelectedBookId(null)}
        />
      )}
    </div>
  );
}
