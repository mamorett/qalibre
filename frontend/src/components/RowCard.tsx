import { Card, Tag, Button, Checkbox } from "@blueprintjs/core";
import { useSearchParams } from "react-router-dom";
import { BookRow } from "../types/book";

interface RowCardProps {
  book: BookRow;
  index: number;
  onClick: () => void;
  onToggleRead?: () => void;
  onToggleArchived?: () => void;
  onRemove?: () => void;
  onDelete?: () => void;
  selectable?: boolean;
  selected?: boolean;
  onToggleSelect?: () => void;
}

/** Cover-size presets. The `key` matches both the URL ?cover= value and the
 *  backend /cover/<id>/<resolution> tier (sm/md/lg). */
const COVER_SIZES = {
  sm: { width: 70, height: 105 },
  md: { width: 110, height: 165 },
  lg: { width: 160, height: 240 },
} as const;

type CoverSize = keyof typeof COVER_SIZES;

export function RowCard({
  book,
  index,
  onClick,
  onToggleRead = () => {},
  onRemove,
  onDelete,
  selectable = false,
  selected = false,
  onToggleSelect = () => {},
}: RowCardProps) {
  const [searchParams] = useSearchParams();
  const requested = (searchParams.get("cover") || "md") as CoverSize;
  const coverKey: CoverSize = COVER_SIZES[requested] ? requested : "md";
  const cover = COVER_SIZES[coverKey];

  // Parse tags list if it's a comma-separated string
  const tagsList = book.tags ? book.tags.split(",").map(t => t.trim()).filter(Boolean).slice(0, 3) : [];

  return (
    <Card interactive onClick={selectable ? onToggleSelect : onClick} className="bp6-card" style={{ display: "flex", gap: "1rem", padding: "1rem" }}>
      {/* Cover Image */}
      <div style={{ flexShrink: 0, width: `${cover.width}px`, height: `${cover.height}px`, backgroundColor: "var(--bg-secondary)", border: "1px solid var(--border-light)", overflow: "hidden" }}>
        <img
          src={book.has_cover ? `/cover/${book.id}/${coverKey}` : "/static/generic_cover.jpg"}
          alt={book.title}
          style={{ width: "100%", height: "100%", objectFit: "cover" }}
          loading="lazy"
          onError={(e) => {
            (e.target as HTMLImageElement).src = "/static/generic_cover.jpg";
          }}
        />
      </div>

      {/* Book Metadata */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "0.25rem" }}>
          {/* Row Number */}
          <h5 style={{ color: "var(--accent-primary)", margin: 0, paddingRight: "0.5rem" }}>
            {index}
          </h5>
          
          {/* Toggle Actions */}
          <div style={{ display: "flex", gap: "0.5rem" }} onClick={(e) => e.stopPropagation()}>
            {selectable ? (
              <Checkbox
                checked={selected}
                onChange={onToggleSelect}
                style={{ margin: 0 }}
              />
            ) : onRemove ? (
              <Button
                icon="trash"
                variant="outlined"
                title="Remove from Dataset"
                onClick={onRemove}
                intent="danger"
                style={{ padding: "2px 6px", minHeight: "24px" }}
              />
            ) : (
              <>
                <Button
                  icon={book.read_status ? "eye-open" : "eye-off"}
                  variant="minimal"
                  title={book.read_status ? "Mark Unread" : "Mark Read"}
                  onClick={onToggleRead}
                  className={book.read_status ? "bp6-intent-success" : ""}
                  style={{ padding: "2px 6px", minHeight: "24px" }}
                />
                <Button
                  icon="download"
                  variant="minimal"
                  title="Download"
                  disabled={!book.formats || book.formats.length === 0}
                  onClick={(e) => {
                    e.stopPropagation();
                    if (book.formats && book.formats.length > 0) {
                      const fileExt = book.formats[0].format.toLowerCase();
                      window.open(`/download/${book.id}/${fileExt}`, "_blank");
                    }
                  }}
                  style={{ padding: "2px 6px", minHeight: "24px" }}
                />
                {onDelete && (
                  <Button
                    icon="trash"
                    variant="minimal"
                    title="Delete Book"
                    onClick={onDelete}
                    intent="danger"
                    style={{ padding: "2px 6px", minHeight: "24px" }}
                  />
                )}
              </>
            )}
          </div>
        </div>

        {/* Title */}
        <div className="field-value" style={{ fontWeight: "bold", fontSize: "1.05rem", lineHeight: "1.25", marginBottom: "0.25rem", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
          {book.title}
        </div>

        {/* Authors */}
        <div style={{ fontStyle: "italic", color: "var(--text-secondary)", fontSize: "0.85rem", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis", marginBottom: "0.25rem" }}>
          {book.authors || "Unknown Author"}
        </div>

        {/* Series */}
        {book.series && (
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.5rem", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
            {book.series} ({book.series_index})
          </div>
        )}

        {/* Tags and Conversion Status */}
        <div style={{ marginTop: "auto", display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: "4px" }}>
          <div style={{ display: "flex", flexWrap: "wrap", gap: "4px" }}>
            {tagsList.map(tag => (
              <Tag key={tag} className="bp6-tag">
                {tag}
              </Tag>
            ))}
          </div>

          {(book.is_converted_plain !== undefined || book.is_converted_chunked !== undefined) && (
            <div style={{ display: "flex", gap: "6px" }}>
              <Tag
                intent={book.is_converted_plain ? "success" : "warning"}
                minimal
                icon={book.is_converted_plain ? "tick" : "warning-sign"}
                title={book.is_converted_plain ? "Plain Markdown Converted" : "Plain Markdown Not Converted"}
                style={{ fontSize: "0.75rem", fontFamily: "Space Mono, monospace" }}
              >
                MD
              </Tag>
              <Tag
                intent={book.is_converted_chunked ? "success" : "warning"}
                minimal
                icon={book.is_converted_chunked ? "tick" : "warning-sign"}
                title={book.is_converted_chunked ? "Chunked Markdown Converted" : "Chunked Markdown Not Converted"}
                style={{ fontSize: "0.75rem", fontFamily: "Space Mono, monospace" }}
              >
                Chunked
              </Tag>
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}
