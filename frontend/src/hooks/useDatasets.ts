import { useQuery, useMutation, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { datasetsApi, BookListQ } from "../api/client";
import { CreateDatasetPayload, ExportPayload } from "../types/dataset";

export function useDatasets(search?: string) {
  return useQuery({
    queryKey: ["datasets", search],
    queryFn: () => datasetsApi.list(search),
  });
}

export function useDataset(id: number) {
  return useQuery({
    queryKey: ["dataset", id],
    queryFn: () => datasetsApi.get(id),
    enabled: !isNaN(id),
  });
}

export function useDatasetBooks(id: number, q: BookListQ) {
  return useQuery({
    queryKey: ["dataset-books", id, q],
    queryFn: () => datasetsApi.books(id, q),
    placeholderData: keepPreviousData,
    enabled: !isNaN(id),
  });
}

export function useAvailableBooks(id: number, q: BookListQ) {
  return useQuery({
    queryKey: ["available-books", id, q],
    queryFn: () => datasetsApi.available(id, q),
    placeholderData: keepPreviousData,
    enabled: !isNaN(id),
  });
}

export function useCreateDataset() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (p: CreateDatasetPayload) => datasetsApi.create(p),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["datasets"] });
    },
  });
}

export function useUpdateDataset(id: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (p: Partial<CreateDatasetPayload>) => datasetsApi.update(id, p),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["datasets"] });
      queryClient.invalidateQueries({ queryKey: ["dataset", id] });
    },
  });
}

export function useDeleteDataset() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => datasetsApi.remove(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["datasets"] });
    },
  });
}

export function useAddBooksToDataset(id: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (bookIDs: number[]) => datasetsApi.addBooks(id, { book_ids: bookIDs }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["dataset", id] });
      queryClient.invalidateQueries({ queryKey: ["dataset-books", id] });
      queryClient.invalidateQueries({ queryKey: ["available-books", id] });
    },
  });
}

export function useRemoveBookFromDataset(id: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (bookID: number) => datasetsApi.removeBook(id, bookID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["dataset", id] });
      queryClient.invalidateQueries({ queryKey: ["dataset-books", id] });
      queryClient.invalidateQueries({ queryKey: ["available-books", id] });
    },
  });
}

export function useExportDataset(id: number) {
  return useMutation({
    mutationFn: (p: ExportPayload) => datasetsApi.export(id, p),
  });
}

export function useTasksStatus(refetchInterval?: number) {
  return useQuery({
    queryKey: ["tasks-status"],
    queryFn: () => datasetsApi.tasks(),
    refetchInterval,
  });
}

export function useCancelTask() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (taskId: string) => datasetsApi.cancelTask(taskId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tasks-status"] });
    },
  });
}
