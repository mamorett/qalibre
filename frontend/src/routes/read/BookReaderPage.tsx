import { useParams, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "../../api/client";
import {
  Button,
  Navbar,
  NavbarGroup,
  NavbarHeading,
  NavbarDivider,
  Alignment,
  Spinner,
  SpinnerSize,
  NonIdealState,
  Tag,
  Tooltip,
} from "@blueprintjs/core";

interface BookFormat {
  id: number;
  format: string;
  size: number;
  name: string;
}

interface BookDetail {
  id: number;
  title: string;
  authors?: string;
  formats?: BookFormat[];
  has_cover?: boolean;
}

export default function BookReaderPage() {
  const { id, format } = useParams<{ id: string; format: string }>();
  const navigate = useNavigate();

  const { data: book, isLoading, isError } = useQuery<BookDetail>({
    queryKey: ["book", id],
    queryFn: () => api<BookDetail>(`/api/v1/book/${id}`),
    enabled: !!id,
  });

  // The backend /read/<id>/<format> route serves the PDF inline with correct headers
  const fileUrl = `/read/${id}/${format}`;
  const downloadUrl = `/download/${id}/${format}`;

  if (isLoading) {
    return (
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          alignItems: "center",
          height: "100vh",
          gap: "1rem",
          background: "var(--bg-primary, #1a1a2e)",
          color: "var(--text-primary, #e0e0e0)",
        }}
      >
        <Spinner size={SpinnerSize.LARGE} intent="primary" />
        <p style={{ margin: 0, opacity: 0.7 }}>Loading book…</p>
      </div>
    );
  }

  if (isError || !book) {
    return (
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          height: "100vh",
          background: "var(--bg-primary, #1a1a2e)",
        }}
      >
        <Navbar>
          <NavbarGroup align={Alignment.START}>
            <Button icon="arrow-left" variant="minimal" onClick={() => navigate("/spa")}>
              Back to Library
            </Button>
          </NavbarGroup>
        </Navbar>
        <div style={{ flex: 1, display: "flex", alignItems: "center", justifyContent: "center" }}>
          <NonIdealState
            icon="error"
            title="Could not load book"
            description={`Unable to fetch details for book ID ${id}. It may not exist or you may not have access.`}
            action={
              <Button intent="primary" onClick={() => navigate("/spa")}>
                Return to Library
              </Button>
            }
          />
        </div>
      </div>
    );
  }

  const formatUpper = (format ?? "").toUpperCase();

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100vh",
        width: "100vw",
        overflow: "hidden",
        position: "fixed",
        top: 0,
        left: 0,
        background: "var(--bg-primary, #1a1a2e)",
      }}
    >
      {/* Top Navbar */}
      <Navbar style={{ flexShrink: 0, zIndex: 10 }}>
        <NavbarGroup align={Alignment.START}>
          <Tooltip content="Back to library" placement="bottom">
            <Button
              icon="arrow-left"
              variant="minimal"
              onClick={() => navigate("/spa")}
              style={{ marginRight: "0.5rem" }}
            />
          </Tooltip>
          <NavbarDivider />
          <NavbarHeading style={{ fontSize: "1rem", fontWeight: 600, maxWidth: "50vw", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {book.title}
          </NavbarHeading>
          {book.authors && (
            <span
              style={{
                fontSize: "0.875rem",
                opacity: 0.7,
                marginLeft: "0.5rem",
                fontStyle: "italic",
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
                maxWidth: "20vw",
              }}
            >
              by {book.authors}
            </span>
          )}
          <Tag minimal style={{ marginLeft: "0.75rem" }}>
            {formatUpper}
          </Tag>
        </NavbarGroup>

        <NavbarGroup align={Alignment.END}>
          <Tooltip content="Download file" placement="bottom">
            <Button
              icon="download"
              variant="minimal"
              onClick={() => window.open(downloadUrl, "_blank")}
            >
              Download
            </Button>
          </Tooltip>
          <NavbarDivider />
          <Tooltip content="Open in browser's PDF viewer" placement="bottom">
            <Button
              icon="share"
              variant="minimal"
              onClick={() => window.open(fileUrl, "_blank")}
            >
              Open in Tab
            </Button>
          </Tooltip>
        </NavbarGroup>
      </Navbar>

      {/* PDF Viewer */}
      <div style={{ flex: 1, width: "100%", overflow: "hidden", background: "#525659" }}>
        <object
          data={fileUrl}
          type="application/pdf"
          style={{
            width: "100%",
            height: "100%",
            border: "none",
            display: "block",
          }}
          aria-label={`PDF viewer for ${book.title}`}
        >
          {/* Fallback for browsers that cannot display PDFs inline */}
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              height: "100%",
              gap: "1rem",
              color: "#fff",
              background: "#525659",
            }}
          >
            <NonIdealState
              icon="document"
              title="PDF viewer not available"
              description="Your browser cannot display this PDF inline. Use the button below to open or download it."
              action={
                <div style={{ display: "flex", gap: "0.75rem" }}>
                  <Button intent="primary" icon="share" onClick={() => window.open(fileUrl, "_blank")}>
                    Open PDF
                  </Button>
                  <Button icon="download" onClick={() => window.open(downloadUrl, "_blank")}>
                    Download
                  </Button>
                </div>
              }
            />
          </div>
        </object>
      </div>
    </div>
  );
}
