export type MetadataValueType = "string" | "number" | "timestamp" | "path";

export interface MetadataItem {
  key: string;
  value: string;
  value_type: MetadataValueType;
}

export interface DatasetSummary {
  id: number;
  uuid: string;
  name: string;
  description: string;
  book_count: number;
  created: string;
  last_modified: string;
}

export interface DatasetDetail extends DatasetSummary {
  metadata: MetadataItem[];
  export_directory: string;
}

export interface CreateDatasetPayload {
  name: string;
  description?: string;
  metadata?: MetadataItem[];
  export_directory?: string;
}

export interface AddBooksPayload {
  book_ids: number[];
}

export interface ExportPayload {
  path: string;
  force?: boolean;
}

export interface ExportChunkedPayload {
  path?: string;
  chunk_size: number;
  chunk_overlap: number;
  force?: boolean;
}
