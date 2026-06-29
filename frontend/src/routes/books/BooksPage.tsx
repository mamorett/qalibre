import { useSearchParams } from "react-router-dom";
import { useBooks } from "../../hooks/useBooks";
import { RowCard } from "../../components/RowCard";
import { BookDetailDialog } from "../../components/BookDetailDialog";
import { Pagination } from "../../components/Pagination";
import { Spinner, NonIdealState, Button, Alert } from "@blueprintjs/core";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../api/client";
import { useState } from "react";
import { useApp } from "../../context/AppContext";

export default function BooksPage() {
  const { user } = useApp();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const [selectedBookId, setSelectedBookId] = useState<number | null>(null);
  const [deleteBookId, setDeleteBookId] = useState<number | null>(null);

  const canDelete = !!(user?.role_admin || user?.role_delete_books);

  // Extract query parameters
  const search = searchParams.get("q") || "";
  const sort = searchParams.get("sort") || "timestamp";
  const order = searchParams.get("order") || "desc";
  const limit = parseInt(searchParams.get("limit") || "60");
  const offset = parseInt(searchParams.get("offset") || "0");
  const cover = searchParams.get("cover") || "md";
  // Large covers need a slightly wider grid cell so the metadata column isn't cramped.
  const gridMin = cover === "lg" ? 360 : 320;

  // Query books
  const { data, isLoading, isError, refetch } = useBooks({
    offset,
    limit,
    search,
    sort,
    order,
  });

  // Mutations
  const toggleReadMutation = useMutation({
    mutationFn: (bookId: number) => api(`/ajax/toggleread/${bookId}`, { method: "POST" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["books"] });
    },
  });

  const toggleArchivedMutation = useMutation({
    mutationFn: (bookId: number) => api(`/ajax/togglearchived/${bookId}`, { method: "POST" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["books"] });
    },
  });

  const deleteBookMutation = useMutation({
    mutationFn: (bookId: number) => api(`/api/v1/book/${bookId}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["books"] });
    },
  });

  const handlePageChange = (newOffset: number) => {
    const params = new URLSearchParams(searchParams);
    params.set("offset", String(newOffset));
    setSearchParams(params);
  };

  const goToPage = (page: number) => handlePageChange(Math.max(0, (page - 1) * limit));

  if (isLoading) {
    return (
      <div style={{ display: "flex", justifyContent: "center", alignItems: "center", minHeight: "50vh" }}>
        <Spinner size={50} intent="primary" />
      </div>
    );
  }

  if (isError || !data) {
    return (
      <NonIdealState
        icon="error"
        title="Failed to Load Books"
        description="There was an error communicating with the Qalibre server."
        action={<Button icon="refresh" onClick={() => refetch()}>Retry</Button>}
      />
    );
  }

  const { rows, total } = data;

  if (rows.length === 0) {
    return (
      <NonIdealState
        icon="search"
        title="No Books Found"
        description={search ? `No books matched the query: "${search}"` : "The Calibre database is currently empty."}
      />
    );
  }

  const currentPage = Math.floor(offset / limit) + 1;
  const totalPages = Math.ceil(total / limit);
  const startIdx = offset + 1;
  const endIdx = Math.min(offset + limit, total);

  const paginationProps = {
    currentPage,
    totalPages,
    total,
    startIdx,
    endIdx,
    onPageChange: goToPage,
  };

  return (
    <div style={{ display: "flex", flexDirection: "column" }}>
      {/* Sticky top pagination bar — always visible while scrolling */}
      {totalPages > 1 && (
        <div className="pagination-top">
          <Pagination {...paginationProps} />
        </div>
      )}

      {/* Grid of Books */}
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
        {rows.map((book, idx) => (
          <RowCard
            key={book.id}
            book={book}
            index={offset + idx + 1}
            onClick={() => setSelectedBookId(book.id)}
            onToggleRead={() => toggleReadMutation.mutate(book.id)}
            onToggleArchived={() => toggleArchivedMutation.mutate(book.id)}
            onDelete={canDelete ? () => setDeleteBookId(book.id) : undefined}
          />
        ))}
      </div>

      {/* Detail Dialog */}
      {selectedBookId !== null && (
        <BookDetailDialog
          bookId={selectedBookId}
          isOpen={selectedBookId !== null}
          onClose={() => setSelectedBookId(null)}
        />
      )}

      {/* Delete Confirmation Alert */}
      <Alert
        cancelButtonText="Cancel"
        confirmButtonText="Delete"
        icon="trash"
        intent="danger"
        isOpen={deleteBookId !== null}
        loading={deleteBookMutation.isPending}
        onCancel={() => setDeleteBookId(null)}
        onConfirm={async () => {
          if (deleteBookId !== null) {
            try {
              await deleteBookMutation.mutateAsync(deleteBookId);
            } catch (err) {
              console.error("Failed to delete book", err);
            } finally {
              setDeleteBookId(null);
            }
          }
        }}
        style={{ borderRadius: 0, backgroundColor: "var(--bg-primary)" }}
      >
        <p>
          Are you sure you want to permanently delete this book? This will remove all database metadata and delete the files from your Calibre library storage.
        </p>
      </Alert>
    </div>
  );
}
