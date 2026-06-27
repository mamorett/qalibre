export interface BookRow {
  id: number;
  title: string;
  sort?: string;
  author_sort?: string;
  timestamp?: string;
  pubdate?: string;
  series_index?: string;
  last_modified?: string;
  path?: string;
  has_cover?: number | boolean;
  uuid?: string;
  authors?: string;
  tags?: string;
  series?: string;
  publishers?: string;
  languages?: string;
  is_archived?: boolean;
  read_status?: boolean;
  comments?: string;
  rating?: number;
}

export interface BookListResponse {
  total: number;
  totalNotFiltered: number;
  rows: BookRow[];
}
