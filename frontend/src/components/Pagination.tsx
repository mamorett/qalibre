import { Button, ButtonGroup, Slider } from "@blueprintjs/core";
import { useEffect, useState } from "react";

interface PaginationProps {
  /** 1-indexed current page */
  currentPage: number;
  /** Total number of pages (>= 1) */
  totalPages: number;
  /** Total book count */
  total: number;
  /** 1-indexed start row shown on this page */
  startIdx: number;
  /** 1-indexed end row shown on this page */
  endIdx: number;
  /** Navigate to a 1-indexed page */
  onPageChange: (page: number) => void;
}

/**
 * Shared pagination control: prev/next buttons, a readable slide-to-page Slider,
 * and an inline jump-to-page input inside the button group.
 */
export function Pagination({
  currentPage,
  totalPages,
  total,
  startIdx,
  endIdx,
  onPageChange,
}: PaginationProps) {
  const clamp = (p: number) => Math.max(1, Math.min(p, totalPages));
  const go = (p: number) => {
    const target = clamp(p);
    if (target !== currentPage) onPageChange(target);
  };

  const [jumpText, setJumpText] = useState(String(currentPage));

  useEffect(() => {
    setJumpText(String(currentPage));
  }, [currentPage]);

  const handlePageSubmit = () => {
    const parsed = parseInt(jumpText, 10);
    if (!Number.isNaN(parsed) && parsed >= 1 && parsed <= totalPages) {
      go(parsed);
    } else {
      setJumpText(String(currentPage));
    }
  };

  return (
    <div
      className="pagination-bar"
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        width: "100%",
        boxSizing: "border-box",
        padding: "0 0.5rem",
        gap: "1.5rem",
      }}
    >
      <div className="pagination-text" style={{ whiteSpace: "nowrap" }}>
        Showing {startIdx}–{endIdx} of {total} books
      </div>

      <div style={{ flex: "1 1 300px", maxWidth: "600px", minWidth: "200px", display: "flex", alignItems: "center" }}>
        <PageSlider
          currentPage={currentPage}
          totalPages={totalPages}
          onNavigate={(p) => go(p)}
        />
      </div>

      <ButtonGroup>
        <Button
          icon="chevron-left"
          disabled={currentPage <= 1}
          onClick={() => go(currentPage - 1)}
          style={{ width: "110px" }}
        >
          Previous
        </Button>
        <div
          className="bp6-button pagination-page-number"
          style={{
            display: "inline-flex",
            alignItems: "center",
            cursor: "default",
            pointerEvents: "auto",
            padding: "0 0.75rem",
            background: "var(--bg-primary)",
            borderColor: "var(--border-light)",
            height: "30px",
          }}
        >
          <span style={{ fontSize: "0.75rem", textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--text-secondary)" }}>Page</span>
          <input
            type="text"
            value={jumpText}
            onChange={(e) => {
              const val = e.target.value.replace(/\D/g, "");
              setJumpText(val);
            }}
            onBlur={handlePageSubmit}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                handlePageSubmit();
                e.currentTarget.blur();
              }
            }}
            style={{
              width: "35px",
              height: "20px",
              textAlign: "center",
              margin: "0 0.4rem",
              border: "1px solid var(--border-color)",
              borderRadius: 0,
              background: "var(--bg-primary)",
              color: "var(--text-primary)",
              fontFamily: "var(--font-mono)",
              fontSize: "0.75rem",
              padding: 0,
            }}
          />
          <span style={{ fontSize: "0.75rem", textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--text-secondary)" }}>of {totalPages}</span>
        </div>
        <Button
          icon="chevron-right"
          disabled={currentPage >= totalPages}
          onClick={() => go(currentPage + 1)}
          style={{ width: "110px" }}
        >
          Next
        </Button>
      </ButtonGroup>
    </div>
  );
}

function PageSlider({
  currentPage,
  totalPages,
  onNavigate,
}: {
  currentPage: number;
  totalPages: number;
  onNavigate: (page: number) => void;
}) {
  const [dragValue, setDragValue] = useState<number | null>(null);
  const max = Math.max(1, totalPages);

  return (
    <Slider
      min={1}
      max={max}
      stepSize={1}
      value={dragValue ?? currentPage}
      labelRenderer={(v) => `Page ${v}`}
      onChange={(v) => setDragValue(v)}
      onRelease={(v) => {
        setDragValue(null);
        onNavigate(Math.max(1, Math.min(v, totalPages)));
      }}
      className="bp6-slider"
    />
  );
}
