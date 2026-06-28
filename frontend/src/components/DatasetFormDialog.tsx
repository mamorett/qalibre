import React, { useState, useEffect } from "react";
import {
  Dialog,
  DialogBody,
  DialogFooter,
  Button,
  InputGroup,
  TextArea,
  HTMLSelect,
  FormGroup,
} from "@blueprintjs/core";
import { DatasetDetail, MetadataItem, MetadataValueType } from "../types/dataset";
import { useCreateDataset, useUpdateDataset } from "../hooks/useDatasets";

interface DatasetFormDialogProps {
  isOpen: boolean;
  mode: "create" | "edit";
  dataset?: DatasetDetail;
  onClose: () => void;
  onSuccess: (dataset: DatasetDetail) => void;
}

interface LocalMetadataItem {
  key: string;
  value: string;
  value_type: MetadataValueType;
  keyError?: string;
  valueError?: string;
}

export function DatasetFormDialog({
  isOpen,
  mode,
  dataset,
  onClose,
  onSuccess,
}: DatasetFormDialogProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [metadata, setMetadata] = useState<LocalMetadataItem[]>([]);

  const createMutation = useCreateDataset();
  const updateMutation = useUpdateDataset(dataset?.id || 0);

  useEffect(() => {
    if (isOpen) {
      if (mode === "edit" && dataset) {
        setName(dataset.name);
        setDescription(dataset.description || "");
        setMetadata(
          dataset.metadata.map((m) => ({
            key: m.key,
            value: m.value,
            value_type: m.value_type,
          }))
        );
      } else {
        setName("");
        setDescription("");
        setMetadata([]);
      }
    }
  }, [isOpen, mode, dataset]);

  const handleMetadataChange = (idx: number, field: keyof LocalMetadataItem, val: string) => {
    setMetadata((prev) =>
      prev.map((item, i) => {
        if (i === idx) {
          const newItem = { ...item, [field]: val };
          if (field === "key") newItem.keyError = undefined;
          if (field === "value" || field === "value_type") newItem.valueError = undefined;
          return newItem;
        }
        return item;
      })
    );
  };

  const addMetadataRow = () => {
    setMetadata((prev) => [...prev, { key: "", value: "", value_type: "string" }]);
  };

  const removeMetadataRow = (idx: number) => {
    setMetadata((prev) => prev.filter((_, i) => i !== idx));
  };

  const validate = (): boolean => {
    let hasError = false;
    const updated = metadata.map((item) => {
      const key = item.key.trim();
      const val = item.value.trim();
      const type = item.value_type;
      let keyError: string | undefined;
      let valueError: string | undefined;

      if (!key) {
        keyError = "Key is required";
        hasError = true;
      }
      if (!val) {
        valueError = "Value is required";
        hasError = true;
      } else {
        if (type === "number") {
          if (isNaN(Number(val))) {
            valueError = "Must be a valid number";
            hasError = true;
          }
        } else if (type === "timestamp") {
          if (isNaN(Date.parse(val))) {
            valueError = "Invalid date format";
            hasError = true;
          }
        }
      }

      return { ...item, keyError, valueError };
    });

    if (hasError) {
      setMetadata(updated);
    }
    return !hasError;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (name.trim() === "") return;
    if (!validate()) return;

    const cleanMetadata: MetadataItem[] = metadata.map((m) => ({
      key: m.key.trim(),
      value: m.value.trim(),
      value_type: m.value_type,
    }));

    if (mode === "create") {
      createMutation.mutate(
        {
          name: name.trim(),
          description: description.trim(),
          metadata: cleanMetadata,
        },
        {
          onSuccess: (data) => {
            onSuccess(data);
            onClose();
          },
        }
      );
    } else if (dataset) {
      updateMutation.mutate(
        {
          name: name.trim(),
          description: description.trim(),
          metadata: cleanMetadata,
        },
        {
          onSuccess: (data) => {
            onSuccess(data);
            onClose();
          },
        }
      );
    }
  };

  const isLoading = createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog
      isOpen={isOpen}
      onClose={onClose}
      title={mode === "create" ? "Create New Dataset" : "Edit Dataset"}
      style={{
        borderRadius: 0,
        fontFamily: "Space Mono, monospace",
        backgroundColor: "var(--bg-secondary)",
        border: "1px solid var(--border-color)",
        width: "600px",
        maxWidth: "95vw",
      }}
    >
      <form onSubmit={handleSubmit}>
        <DialogBody>
          <FormGroup
            label="Dataset Name"
            labelInfo="(required)"
            labelFor="dataset-name"
            style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem" }}
          >
            <InputGroup
              id="dataset-name"
              placeholder="e.g., LLaMA Fine-tuning Dataset"
              value={name}
              onChange={(e) => setName(e.target.value)}
              style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
              required
            />
          </FormGroup>

          <FormGroup
            label="Description"
            labelFor="dataset-desc"
            style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem" }}
          >
            <TextArea
              id="dataset-desc"
              placeholder="A brief description of this dataset and its purpose..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              style={{
                borderRadius: 0,
                border: "1px solid var(--border-color)",
                fontFamily: "sans-serif",
                width: "100%",
                minHeight: "80px",
              }}
            />
          </FormGroup>

          <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem", marginTop: "1rem" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span style={{ fontSize: "0.75rem", fontWeight: "bold", textTransform: "uppercase" }}>
                Metadata Fields
              </span>
              <Button icon="add" variant="minimal" onClick={addMetadataRow} style={{ fontFamily: "Space Mono, monospace" }}>
                Add Field
              </Button>
            </div>

            {metadata.map((item, idx) => (
              <div
                key={idx}
                style={{
                  display: "flex",
                  flexDirection: "column",
                  gap: "0.25rem",
                  borderBottom: "1px dashed var(--border-color)",
                  paddingBottom: "0.75rem",
                }}
              >
                <div style={{ display: "flex", gap: "0.5rem", alignItems: "flex-start" }}>
                  <div style={{ flex: 2 }}>
                    <InputGroup
                      placeholder="Key"
                      value={item.key}
                      onChange={(e) => handleMetadataChange(idx, "key", e.target.value)}
                      intent={item.keyError ? "danger" : "none"}
                      style={{ borderRadius: 0, fontFamily: "sans-serif" }}
                    />
                  </div>
                  <div style={{ flex: 1.5 }}>
                    <HTMLSelect
                      value={item.value_type}
                      onChange={(e) => handleMetadataChange(idx, "value_type", e.target.value as MetadataValueType)}
                      style={{ borderRadius: 0, width: "100%", fontFamily: "Space Mono, monospace" }}
                      options={[
                        { label: "String", value: "string" },
                        { label: "Number", value: "number" },
                        { label: "Timestamp", value: "timestamp" },
                        { label: "Path", value: "path" },
                      ]}
                    />
                  </div>
                  <div style={{ flex: 3 }}>
                    <InputGroup
                      placeholder={
                        item.value_type === "timestamp"
                          ? "e.g., 2026-06-28 09:00:00"
                          : item.value_type === "path"
                          ? "/exports/my-dataset"
                          : "Value"
                      }
                      value={item.value}
                      onChange={(e) => handleMetadataChange(idx, "value", e.target.value)}
                      intent={item.valueError ? "danger" : "none"}
                      style={{ borderRadius: 0, fontFamily: "sans-serif" }}
                    />
                  </div>
                  <Button icon="cross" intent="danger" variant="minimal" onClick={() => removeMetadataRow(idx)} />
                </div>
                {(item.keyError || item.valueError) && (
                  <div style={{ display: "flex", gap: "1rem", fontSize: "0.75rem", color: "var(--accent-red)", paddingLeft: "0.25rem" }}>
                    {item.keyError && <span>{item.keyError}</span>}
                    {item.valueError && <span>{item.valueError}</span>}
                  </div>
                )}
              </div>
            ))}
          </div>
        </DialogBody>
        <DialogFooter
          actions={
            <div style={{ display: "flex", gap: "1rem" }}>
              <Button onClick={onClose} style={{ borderRadius: 0, fontFamily: "Space Mono, monospace", textTransform: "uppercase" }}>
                Cancel
              </Button>
              <Button
                type="submit"
                intent="primary"
                loading={isLoading}
                disabled={name.trim() === ""}
                style={{ borderRadius: 0, fontFamily: "Space Mono, monospace", textTransform: "uppercase" }}
              >
                {mode === "create" ? "Create" : "Save Changes"}
              </Button>
            </div>
          }
        />
      </form>
    </Dialog>
  );
}
