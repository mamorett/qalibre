import React, { useState, useEffect, useRef, useImperativeHandle, forwardRef } from "react";
import { Button, Icon, Spinner } from "@blueprintjs/core";
import { useQueryClient } from "@tanstack/react-query";
import { useApp } from "../context/AppContext";

export interface UploadManagerRef {
  triggerUpload: () => void;
}

interface UploadItem {
  id: string;
  file: File;
  progress: number;
  status: "pending" | "uploading" | "success" | "error";
  errorMsg?: string;
}

const ALLOWED_EXTENSIONS = [
  "epub", "pdf", "txt", "mobi", "azw3", "docx", "rtf", "odt", "html", "fb2", "djvu", "cbz", "cbr"
];

function getFileExtension(filename: string): string {
  return filename.split(".").pop()?.toLowerCase() || "";
}

function formatBytes(bytes: number, decimals = 2) {
  if (bytes === 0) return "0 Bytes";
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ["Bytes", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + " " + sizes[i];
}

function uploadFile(
  file: File,
  csrfToken: string | null,
  onProgress: (percent: number) => void
): Promise<any> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/upload");
    xhr.withCredentials = true;

    if (csrfToken) {
      xhr.setRequestHeader("X-CSRFToken", csrfToken);
    }
    xhr.setRequestHeader("Accept", "application/json");
    xhr.setRequestHeader("X-Requested-With", "XMLHttpRequest");

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        const percent = Math.round((event.loaded / event.total) * 100);
        onProgress(percent);
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          const res = JSON.parse(xhr.responseText);
          resolve(res);
        } catch {
          resolve(xhr.responseText);
        }
      } else {
        let errMsg = `Upload failed (${xhr.status})`;
        try {
          const res = JSON.parse(xhr.responseText);
          if (res.error) errMsg = res.error;
        } catch {}
        reject(new Error(errMsg));
      }
    };

    xhr.onerror = () => {
      reject(new Error("Network error during upload"));
    };

    const formData = new FormData();
    formData.append("btn-upload", file);
    xhr.send(formData);
  });
}

export const UploadManager = forwardRef<UploadManagerRef, {}>((_, ref) => {
  const { user, csrfToken } = useApp();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  
  const [queue, setQueue] = useState<UploadItem[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [dragActive, setDragActive] = useState(false);
  const dragCounter = useRef(0);

  const canUpload = !!(user?.role_admin || user?.role_upload);

  useImperativeHandle(ref, () => ({
    triggerUpload: () => {
      fileInputRef.current?.click();
    }
  }));

  const handleFiles = (files: File[]) => {
    if (!canUpload) return;

    const newItems = files.map((file) => {
      const ext = getFileExtension(file.name);
      const isValid = ALLOWED_EXTENSIONS.includes(ext);
      return {
        id: `${file.name}-${Date.now()}-${Math.random()}`,
        file,
        progress: 0,
        status: isValid ? "pending" : "error",
        errorMsg: isValid ? undefined : `Unsupported format (.${ext})`,
      } as UploadItem;
    });

    setQueue((prev) => [...prev, ...newItems]);
    setIsOpen(true);
    setIsCollapsed(false);
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      handleFiles(Array.from(e.target.files));
      // Clear input so the same file can be selected again
      e.target.value = "";
    }
  };

  const handleOverlayDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const handleOverlayDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    dragCounter.current = 0;
  };

  const handleOverlayDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    dragCounter.current = 0;

    if (e.dataTransfer?.files && e.dataTransfer.files.length > 0) {
      handleFiles(Array.from(e.dataTransfer.files));
    }
  };

  useEffect(() => {
    if (!canUpload) {
      setDragActive(false);
      dragCounter.current = 0;
      return;
    }

    const handleWindowDragEnter = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      dragCounter.current++;
      setDragActive(true);
    };

    const handleWindowDragLeave = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      dragCounter.current--;
      if (dragCounter.current <= 0) {
        setDragActive(false);
        dragCounter.current = 0;
      }
    };

    const handleWindowDragOver = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      if (!dragActive) {
        setDragActive(true);
      }
    };

    const handleWindowDrop = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setDragActive(false);
      dragCounter.current = 0;
    };

    window.addEventListener("dragenter", handleWindowDragEnter);
    window.addEventListener("dragleave", handleWindowDragLeave);
    window.addEventListener("dragover", handleWindowDragOver);
    window.addEventListener("drop", handleWindowDrop);

    return () => {
      window.removeEventListener("dragenter", handleWindowDragEnter);
      window.removeEventListener("dragleave", handleWindowDragLeave);
      window.removeEventListener("dragover", handleWindowDragOver);
      window.removeEventListener("drop", handleWindowDrop);
    };
  }, [canUpload]);

  // Queue runner
  useEffect(() => {
    if (queue.length === 0) return;

    // Is there anything actively uploading?
    const activeUpload = queue.find((item) => item.status === "uploading");
    if (activeUpload) return;

    // Find the next pending upload
    const pendingItem = queue.find((item) => item.status === "pending");
    if (!pendingItem) return;

    // Start uploading it
    setQueue((prev) =>
      prev.map((item) =>
        item.id === pendingItem.id ? { ...item, status: "uploading" } : item
      )
    );

    uploadFile(pendingItem.file, csrfToken, (percent) => {
      setQueue((prev) =>
        prev.map((item) =>
          item.id === pendingItem.id ? { ...item, progress: percent } : item
        )
      );
    })
      .then(() => {
        setQueue((prev) =>
          prev.map((item) =>
            item.id === pendingItem.id
              ? { ...item, status: "success", progress: 100 }
              : item
          )
        );
        // Refresh query
        queryClient.invalidateQueries({ queryKey: ["books"] });
      })
      .catch((err) => {
        setQueue((prev) =>
          prev.map((item) =>
            item.id === pendingItem.id
              ? { ...item, status: "error", errorMsg: err.message || "Upload failed" }
              : item
          )
        );
      });
  }, [queue, csrfToken, queryClient]);

  if (!canUpload) return null;

  const totalCount = queue.length;
  const successCount = queue.filter((item) => item.status === "success").length;
  const errorCount = queue.filter((item) => item.status === "error").length;
  const isFinished = queue.every(
    (item) => item.status === "success" || item.status === "error"
  );
  const isUploading = queue.some((item) => item.status === "uploading");

  const clearQueue = () => {
    if (isUploading) return;
    setQueue([]);
    setIsOpen(false);
  };

  return (
    <>
      {/* Hidden file input */}
      <input
        type="file"
        multiple
        ref={fileInputRef}
        onChange={handleFileChange}
        style={{ display: "none" }}
        accept={ALLOWED_EXTENSIONS.map(ext => `.${ext}`).join(",")}
      />

      {/* Full-screen drag indicator overlay */}
      {dragActive && (
        <div
          className="upload-drag-overlay"
          onDragOver={handleOverlayDragOver}
          onDragLeave={handleOverlayDragLeave}
          onDrop={handleOverlayDrop}
        >
          <div className="upload-drag-container">
            <Icon icon="cloud-upload" size={64} style={{ color: "var(--accent-primary)" }} />
            <div className="upload-drag-title">Drop books to upload</div>
            <div className="upload-drag-subtitle">
              Supports EPUB, PDF, MOBI, TXT, DOCX, and more
            </div>
          </div>
        </div>
      )}

      {/* Floating status widget at the bottom right */}
      {isOpen && queue.length > 0 && (
        <div className={`upload-status-widget ${isCollapsed ? "collapsed" : ""}`}>
          <div className="upload-status-header">
            <h6 className="upload-status-title">
              {isFinished
                ? `Upload complete (${successCount} successful, ${errorCount} failed)`
                : `Uploading ${successCount + errorCount}/${totalCount} books...`}
            </h6>
            <div className="upload-status-actions">
              <Button
                icon={isCollapsed ? "chevron-up" : "chevron-down"}
                variant="minimal"
                onClick={() => setIsCollapsed(!isCollapsed)}
                small
              />
              <Button
                icon="cross"
                variant="minimal"
                disabled={isUploading}
                onClick={clearQueue}
                small
                title="Close and clear queue"
              />
            </div>
          </div>

          {!isCollapsed && (
            <ul className="upload-status-list">
              {queue.map((item) => (
                <li key={item.id} className="upload-status-item">
                  {item.status === "uploading" && <Spinner size={16} />}
                  {item.status === "pending" && (
                    <Icon icon="circle" size={16} style={{ color: "var(--text-muted)", opacity: 0.5 }} />
                  )}
                  {item.status === "success" && (
                    <Icon icon="tick-circle" size={16} intent="success" />
                  )}
                  {item.status === "error" && (
                    <Icon icon="warning-sign" size={16} intent="danger" />
                  )}

                  <div className="upload-item-details">
                    <div className="upload-item-name" title={item.file.name}>
                      {item.file.name}
                    </div>
                    <div className="upload-item-meta">
                      <span>{formatBytes(item.file.size)}</span>
                      {item.status === "uploading" && (
                        <span>{item.progress}%</span>
                      )}
                      {item.status === "success" && (
                        <span style={{ color: "var(--status-success)" }}>Done</span>
                      )}
                      {item.status === "error" && (
                        <span style={{ color: "var(--status-error)" }}>Failed</span>
                      )}
                    </div>
                    {item.status === "uploading" && (
                      <div className="upload-progress-container">
                        <div
                          className="upload-progress-bar"
                          style={{ width: `${item.progress}%` }}
                        />
                      </div>
                    )}
                    {item.status === "error" && item.errorMsg && (
                      <div className="upload-item-error">{item.errorMsg}</div>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </>
  );
});

UploadManager.displayName = "UploadManager";
