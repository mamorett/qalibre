import { useQuery, keepPreviousData } from "@tanstack/react-query";
import { api } from "../api/client";
import { BookListResponse } from "../types/book";

interface UseBooksParams {
  offset: number;
  limit: number;
  search?: string;
  sort?: string;
  order?: string;
}

export function useBooks(params: UseBooksParams) {
  const qs = new URLSearchParams();
  qs.set("offset", String(params.offset));
  qs.set("limit", String(params.limit));
  
  if (params.search) {
    qs.set("search", params.search);
  }
  if (params.sort) {
    qs.set("sort", params.sort);
  }
  if (params.order) {
    qs.set("order", params.order);
  }

  return useQuery({
    queryKey: ["books", params],
    queryFn: () => api<BookListResponse>(`/ajax/listbooks?${qs.toString()}`),
    placeholderData: keepPreviousData,
  });
}
