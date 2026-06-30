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
  s3_endpoint?: string;
  s3_region?: string;
  s3_bucket?: string;
  s3_access_key?: string;
  s3_secret_key?: string;
  s3_use_ssl?: boolean;
  s3_force_path_style?: boolean;
}

export interface CreateDatasetPayload {
  name: string;
  description?: string;
  metadata?: MetadataItem[];
  export_directory?: string;
  s3_endpoint?: string;
  s3_region?: string;
  s3_bucket?: string;
  s3_access_key?: string;
  s3_secret_key?: string;
  s3_use_ssl?: boolean;
  s3_force_path_style?: boolean;
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
