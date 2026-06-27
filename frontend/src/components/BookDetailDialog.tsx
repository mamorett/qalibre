import { Dialog, DialogBody, Spinner, Button, ButtonGroup, Switch, Tag } from "@blueprintjs/core";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useApp } from "../context/AppContext";
import { api } from "../api/client";
import { BookRow } from "../types/book";

interface BookDetailDialogProps {
  bookId: number | null;
  isOpen: boolean;
  onClose: () => void;
}

interface FormatDetail {
  id: number;
  format: string;
  size: number;
  name: string;
}

interface IdentifierDetail {
  type: string;
  val: string;
}

interface ShelfDetail {
  id: number;
  name: string;
  is_public: boolean;
  kobo_sync: boolean;
  count: number;
}

interface BookDetail extends BookRow {
  authors_list?: string[];
  tags_list?: string[];
  publisher?: string;
  formats?: FormatDetail[];
  identifiers?: IdentifierDetail[];
  shelves?: number[];
}

export function BookDetailDialog({ bookId, isOpen, onClose }: BookDetailDialogProps) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { user } = useApp();

  const { data: book, isLoading } = useQuery<BookDetail>({
    queryKey: ["book", bookId],
    queryFn: () => api<BookDetail>(`/api/v1/book/${bookId}`),
    enabled: bookId !== null && isOpen,
  });

  const { data: shelves } = useQuery<ShelfDetail[]>({
    queryKey: ["shelves"],
    queryFn: () => api<ShelfDetail[]>("/api/v1/shelves"),
    enabled: isOpen,
  });

  const toggleReadMutation = useMutation({
    mutationFn: () => api(`/ajax/toggleread/${bookId}`, { method: "POST" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["book", bookId] });
      queryClient.invalidateQueries({ queryKey: ["books"] });
    },
  });

  const toggleArchivedMutation = useMutation({
    mutationFn: () => api(`/ajax/togglearchived/${bookId}`, { method: "POST" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["book", bookId] });
      queryClient.invalidateQueries({ queryKey: ["books"] });
    },
  });

  const addToShelfMutation = useMutation({
    mutationFn: (shelfId: number) => api(`/shelf/add/${shelfId}/${bookId}`, { method: "POST" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["book", bookId] });
      queryClient.invalidateQueries({ queryKey: ["books"] });
    },
  });

  const removeFromShelfMutation = useMutation({
    mutationFn: (shelfId: number) => api(`/shelf/remove/${shelfId}/${bookId}`, { method: "POST" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["book", bookId] });
      queryClient.invalidateQueries({ queryKey: ["books"] });
    },
  });

  if (!isOpen) return null;

  return (
    <Dialog
      title={isLoading ? "Loading Book Details..." : book?.title}
      isOpen={isOpen}
      onClose={onClose}
      style={{ width: "750px", maxWidth: "90%" }}
    >
      <DialogBody>
        {isLoading ? (
          <div style={{ display: "flex", justifyContent: "center", padding: "3rem" }}>
            <Spinner />
          </div>
        ) : book ? (
          <div style={{ display: "flex", flexDirection: "row", gap: "2rem", flexWrap: "wrap" }}>
            {/* Left Cover Column */}
            <div style={{ flex: "0 0 200px", display: "flex", flexDirection: "column", gap: "1rem" }}>
              <img
                className="detail-dialog-img"
                src={book.has_cover ? `/cover/${book.id}/og` : "/static/generic_cover.jpg"}
                alt={book.title}
                style={{ width: "100%", height: "auto", border: "1px solid var(--border-color)" }}
                onError={(e) => {
                  (e.target as HTMLImageElement).src = "/static/generic_cover.jpg";
                }}
              />
              <div style={{ borderTop: "1px solid var(--border-light)", paddingTop: "1rem" }}>
                <Switch
                  checked={!!book.read_status}
                  label="Mark as Read"
                  onChange={() => toggleReadMutation.mutate()}
                  large
                />
                <Switch
                  checked={!!book.is_archived}
                  label="Archived"
                  onChange={() => toggleArchivedMutation.mutate()}
                  large
                />
              </div>
            </div>

            {/* Right Meta Column */}
            <div style={{ flex: 1, minWidth: "300px" }}>
              {/* Title & Author */}
              <div style={{ marginBottom: "1.5rem" }}>
                <h4 style={{ margin: "0 0 0.5rem 0", fontFamily: "var(--font-serif)", textTransform: "none", letterSpacing: "normal", fontSize: "1.8rem" }}>
                  {book.title}
                </h4>
                <div style={{ fontStyle: "italic", fontSize: "1.1rem", color: "var(--text-secondary)" }}>
                  by {book.authors || "Unknown Author"}
                </div>
              </div>

              {/* Field Rows */}
              <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem", marginBottom: "1.5rem" }}>
                {book.series && (
                  <div className="field-row">
                    <b>Series:</b> <span className="field-value">{book.series} #{book.series_index}</span>
                  </div>
                )}
                {book.publisher && (
                  <div className="field-row">
                    <b>Publisher:</b> <span className="field-value">{book.publisher}</span>
                  </div>
                )}
                {book.pubdate && (
                  <div className="field-row">
                    <b>Published:</b> <span className="field-value">{new Date(book.pubdate).toLocaleDateString()}</span>
                  </div>
                )}
                {book.languages && (
                  <div className="field-row">
                    <b>Languages:</b> <span className="field-value">{book.languages}</span>
                  </div>
                )}
                {book.identifiers && book.identifiers.length > 0 && (
                  <div className="field-row" style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                    <b>Identifiers:</b>
                    <div style={{ display: "flex", gap: "0.25rem", flexWrap: "wrap" }}>
                      {book.identifiers.map(ident => (
                        <Tag key={ident.type} className="bp6-tag">
                          {ident.type}:{ident.val}
                        </Tag>
                      ))}
                    </div>
                  </div>
                )}
                {book.tags_list && book.tags_list.length > 0 && (
                  <div className="field-row" style={{ display: "flex", alignItems: "center", gap: "0.5rem", marginTop: "0.5rem" }}>
                    <b>Tags:</b>
                    <div style={{ display: "flex", gap: "0.25rem", flexWrap: "wrap" }}>
                      {book.tags_list.map(tag => (
                        <Tag key={tag} className="bp6-tag bp6-intent-primary">
                          {tag}
                        </Tag>
                      ))}
                    </div>
                  </div>
                )}
              </div>

              {/* Bookshelves UI */}
              {shelves && shelves.length > 0 && (
                <div style={{ marginBottom: "1.5rem" }}>
                  <h6 style={{ marginBottom: "0.5rem" }}>Bookshelves</h6>
                  <div style={{ display: "flex", flexWrap: "wrap", gap: "0.5rem" }}>
                    {shelves.map(shelf => {
                      const inShelf = book.shelves?.includes(shelf.id);
                      return (
                        <Button
                          key={shelf.id}
                          variant="minimal"
                          icon={inShelf ? "star" : "star-empty"}
                          onClick={() => {
                            if (inShelf) {
                              removeFromShelfMutation.mutate(shelf.id);
                            } else {
                              addToShelfMutation.mutate(shelf.id);
                            }
                          }}
                          className={inShelf ? "bp6-intent-primary" : ""}
                          style={inShelf ? { backgroundColor: "var(--bg-tertiary)" } : {}}
                        >
                          {shelf.name}
                        </Button>
                      );
                    })}
                  </div>
                </div>
              )}

              {/* Edit Metadata Button (Admin/Edit role) */}
              {(user?.role_edit || user?.role_admin) && (
                <div style={{ marginBottom: "1.5rem" }}>
                  <Button
                    icon="edit"
                    intent="primary"
                    onClick={() => {
                      onClose();
                      navigate(`/spa/edit/${book.id}`);
                    }}
                  >
                    Edit Metadata
                  </Button>
                </div>
              )}

              {/* Downloads & Reading */}
              {book.formats && book.formats.length > 0 && (
                <div style={{ marginBottom: "1.5rem" }}>
                  <h6 style={{ marginBottom: "0.5rem" }}>Available Formats</h6>
                  <ButtonGroup>
                    {book.formats.map(fmt => {
                      const fileExt = fmt.format.toLowerCase();
                      const downloadUrl = `/download/${book.id}/${fileExt}`;
                      const readUrl = fileExt === "pdf" ? `/spa/read/${book.id}/pdf` : `/read/${book.id}/${fileExt}`;
                      const readable = ["epub", "pdf", "cbr", "cbz", "txt"].includes(fileExt);

                      return (
                        <ButtonGroup key={fmt.id}>
                          <Button
                            icon="download"
                            onClick={() => window.open(downloadUrl, "_blank")}
                          >
                            Download {fmt.format}
                          </Button>
                          {readable && (
                            <Button
                              icon="document-share"
                              intent="success"
                              onClick={() => window.open(readUrl, "_blank")}
                            >
                              Read
                            </Button>
                          )}
                        </ButtonGroup>
                      );
                    })}
                  </ButtonGroup>
                </div>
              )}

              {/* Description */}
              {book.comments && (
                <div style={{ borderTop: "1px solid var(--border-light)", paddingTop: "1rem" }}>
                  <h6 style={{ marginBottom: "0.5rem" }}>Description</h6>
                  <div
                    className="markdown-field-content field-value"
                    dangerouslySetInnerHTML={{ __html: book.comments }}
                    style={{ fontSize: "0.95rem", lineHeight: "1.6" }}
                  />
                </div>
              )}
            </div>
          </div>
        ) : (
          <div>Failed to load book metadata.</div>
        )}
      </DialogBody>
    </Dialog>
  );
}
