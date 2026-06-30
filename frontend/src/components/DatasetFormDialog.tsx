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
  Checkbox,
  RadioGroup,
  Radio,
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
  const [exportDirectory, setExportDirectory] = useState("");
  const [exportDirectoryError, setExportDirectoryError] = useState<string | undefined>(undefined);
  const [metadata, setMetadata] = useState<LocalMetadataItem[]>([]);
  const [storageType, setStorageType] = useState<"local" | "s3">("local");
  const [s3Endpoint, setS3Endpoint] = useState("");
  const [s3Region, setS3Region] = useState("");
  const [s3Bucket, setS3Bucket] = useState("");
  const [s3AccessKey, setS3AccessKey] = useState("");
  const [s3SecretKey, setS3SecretKey] = useState("");
  const [s3UseSSL, setS3UseSSL] = useState(true);
  const [s3ForcePathStyle, setS3ForcePathStyle] = useState(true);

  const createMutation = useCreateDataset();
  const updateMutation = useUpdateDataset(dataset?.id || 0);

  useEffect(() => {
    if (isOpen) {
      setExportDirectoryError(undefined);
      if (mode === "edit" && dataset) {
        setName(dataset.name);
        setDescription(dataset.description || "");
        setExportDirectory(dataset.export_directory || "");
        setMetadata(
          dataset.metadata.map((m) => ({
            key: m.key,
            value: m.value,
            value_type: m.value_type,
          }))
        );
        setS3Endpoint(dataset.s3_endpoint || "");
        setS3Region(dataset.s3_region || "");
        setS3Bucket(dataset.s3_bucket || "");
        setS3AccessKey(dataset.s3_access_key || "");
        setS3SecretKey(dataset.s3_secret_key || "");
        setS3UseSSL(dataset.s3_use_ssl !== false);
        setS3ForcePathStyle(dataset.s3_force_path_style !== false);
        const hasS3 = !!(dataset.s3_endpoint && dataset.s3_endpoint.trim());
        setStorageType(hasS3 ? "s3" : "local");
      } else {
        setName("");
        setDescription("");
        setExportDirectory("");
        setMetadata([]);
        setS3Endpoint("");
        setS3Region("");
        setS3Bucket("");
        setS3AccessKey("");
        setS3SecretKey("");
        setS3UseSSL(true);
        setS3ForcePathStyle(true);
        setStorageType("local");
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

    let exportDirErr: string | undefined;
    if (storageType === "local") {
      const dir = exportDirectory.trim();
      if (dir) {
        const isAbs = dir.startsWith('/') || !!dir.match(/^[A-Za-z]:[\\\/]/);
        if (!isAbs) {
          exportDirErr = "Must be an absolute path";
          hasError = true;
        }
      }
    }
    setExportDirectoryError(exportDirErr);

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

    const payload = {
      name: name.trim(),
      description: description.trim(),
      metadata: cleanMetadata,
      export_directory: storageType === "local" ? exportDirectory.trim() : "",
      s3_endpoint: storageType === "s3" ? s3Endpoint.trim() : "",
      s3_region: storageType === "s3" ? s3Region.trim() : "",
      s3_bucket: storageType === "s3" ? s3Bucket.trim() : "",
      s3_access_key: storageType === "s3" ? s3AccessKey.trim() : "",
      s3_secret_key: storageType === "s3" ? s3SecretKey.trim() : "",
      s3_use_ssl: storageType === "s3" ? s3UseSSL : true,
      s3_force_path_style: storageType === "s3" ? s3ForcePathStyle : true,
    };

    if (mode === "create") {
      createMutation.mutate(payload, {
        onSuccess: (data) => {
          onSuccess(data);
          onClose();
        },
      });
    } else if (dataset) {
      updateMutation.mutate(payload, {
        onSuccess: (data) => {
          onSuccess(data);
          onClose();
        },
      });
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
            label={
              <span style={{ fontSize: "0.75rem", fontWeight: "bold", textTransform: "uppercase" }}>
                Dataset Name
              </span>
            }
            labelInfo="(required)"
            labelFor="dataset-name"
            style={{ marginBottom: "1rem" }}
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
            label={
              <span style={{ fontSize: "0.75rem", fontWeight: "bold", textTransform: "uppercase" }}>
                Description
              </span>
            }
            labelFor="dataset-desc"
            style={{ marginBottom: "1rem" }}
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

          <FormGroup
            label={
              <span style={{ fontSize: "0.75rem", fontWeight: "bold", textTransform: "uppercase" }}>
                Storage Type
              </span>
            }
            style={{ marginBottom: "1rem" }}
          >
            <RadioGroup
              inline
              onChange={(e) => setStorageType(e.currentTarget.value as "local" | "s3")}
              selectedValue={storageType}
            >
              <Radio label="Local Directory" value="local" style={{ fontFamily: "Space Mono, monospace", fontSize: "0.8rem", textTransform: "uppercase" }} />
              <Radio label="S3 Compatible Storage" value="s3" style={{ fontFamily: "Space Mono, monospace", fontSize: "0.8rem", textTransform: "uppercase" }} />
            </RadioGroup>
          </FormGroup>

          {storageType === "local" ? (
            <FormGroup
              label="Export Directory"
              labelInfo="(absolute path, optional)"
              labelFor="dataset-export-dir"
              helperText={exportDirectoryError || "Default directory used for plain & chunked Markdown exports. Subdirectories named after the dataset are created automatically."}
              intent={exportDirectoryError ? "danger" : "none"}
              style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem" }}
            >
              <InputGroup
                id="dataset-export-dir"
                placeholder="e.g., /exports/my-dataset"
                value={exportDirectory}
                onChange={(e) => {
                  setExportDirectory(e.target.value);
                  setExportDirectoryError(undefined);
                }}
                intent={exportDirectoryError ? "danger" : "none"}
                style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
              />
            </FormGroup>
          ) : (
            <div style={{ border: "1px solid var(--border-color)", padding: "1.5rem", marginBottom: "1.5rem", backgroundColor: "var(--bg-tertiary)" }}>
              <FormGroup
                label="S3 Endpoint"
                labelInfo="(required)"
                labelFor="dataset-s3-endpoint"
                style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem" }}
              >
                <InputGroup
                  id="dataset-s3-endpoint"
                  placeholder="s3.amazonaws.com"
                  value={s3Endpoint}
                  onChange={(e) => setS3Endpoint(e.target.value)}
                  style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
                  required
                />
              </FormGroup>

              <div style={{ display: "flex", gap: "1rem" }}>
                <FormGroup
                  label="S3 Region"
                  labelFor="dataset-s3-region"
                  style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem", flex: 1 }}
                >
                  <InputGroup
                    id="dataset-s3-region"
                    placeholder="us-east-1"
                    value={s3Region}
                    onChange={(e) => setS3Region(e.target.value)}
                    style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
                  />
                </FormGroup>

                <FormGroup
                  label="S3 Bucket"
                  labelInfo="(required)"
                  labelFor="dataset-s3-bucket"
                  style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem", flex: 2 }}
                >
                  <InputGroup
                    id="dataset-s3-bucket"
                    placeholder="my-bucket"
                    value={s3Bucket}
                    onChange={(e) => setS3Bucket(e.target.value)}
                    style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
                    required
                  />
                </FormGroup>
              </div>

              <FormGroup
                label="S3 Access Key"
                labelFor="dataset-s3-access-key"
                style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1rem" }}
              >
                <InputGroup
                  id="dataset-s3-access-key"
                  value={s3AccessKey}
                  onChange={(e) => setS3AccessKey(e.target.value)}
                  style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
                />
              </FormGroup>

              <FormGroup
                label="S3 Secret Key"
                labelFor="dataset-s3-secret-key"
                style={{ textTransform: "uppercase", fontSize: "0.75rem", marginBottom: "1.5rem" }}
              >
                <InputGroup
                  id="dataset-s3-secret-key"
                  type="password"
                  value={s3SecretKey}
                  onChange={(e) => setS3SecretKey(e.target.value)}
                  style={{ borderRadius: 0, border: "1px solid var(--border-color)", fontFamily: "sans-serif" }}
                />
              </FormGroup>

              <div style={{ display: "flex", gap: "2rem", marginBottom: "0.5rem" }}>
                <Checkbox
                  label="Use SSL (HTTPS)"
                  checked={s3UseSSL}
                  onChange={(e) => setS3UseSSL(e.target.checked)}
                  style={{ fontSize: "0.8rem", textTransform: "uppercase" }}
                />
                <Checkbox
                  label="Force Path Style"
                  checked={s3ForcePathStyle}
                  onChange={(e) => setS3ForcePathStyle(e.target.checked)}
                  style={{ fontSize: "0.8rem", textTransform: "uppercase" }}
                />
              </div>
            </div>
          )}

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
