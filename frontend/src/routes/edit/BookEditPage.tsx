import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useForm, useFieldArray } from "react-hook-form";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../api/client";
import {
  Button,
  FormGroup,
  InputGroup,
  TextArea,
  Spinner,
  Card,
  Callout,
  HTMLSelect,
  Switch
} from "@blueprintjs/core";
import { BookRow } from "../../types/book";

interface FormatDetail {
  id: number;
  format: string;
  size: number;
  name: string;
}

interface IdentifierDetail {
  type: string;
  val: string;
}

interface CustomColumnDetail {
  id: number;
  label: string;
  name: string;
  datatype: string;
  is_multiple: boolean;
  value: string | null;
  display?: any;
}

interface BookDetail extends BookRow {
  authors_list?: string[];
  tags_list?: string[];
  publisher?: string;
  formats?: FormatDetail[];
  identifiers?: IdentifierDetail[];
  shelves?: number[];
  custom_columns?: CustomColumnDetail[];
}

interface FormValues {
  title: string;
  authors: string;
  series: string;
  series_index: string;
  publisher: string;
  pubdate: string;
  rating: number;
  comments: string;
  languages: string;
  identifiers: { type: string; val: string }[];
  custom_columns: { id: number; value: string }[];
}

export default function BookEditPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [saveError, setSaveError] = useState<string | null>(null);

  // Fetch book details
  const { data: book, isLoading, isError } = useQuery<BookDetail>({
    queryKey: ["book", id],
    queryFn: () => api<BookDetail>(`/api/v1/book/${id}`),
    enabled: !!id,
  });

  const { register, control, handleSubmit, setValue, reset, watch } = useForm<FormValues>({
    defaultValues: {
      title: "",
      authors: "",
      series: "",
      series_index: "1.0",
      publisher: "",
      pubdate: "",
      rating: 0,
      comments: "",
      languages: "",
      identifiers: [],
      custom_columns: [],
    },
  });

  // Helper to translate react-hook-form's standard ref to Blueprint's inputRef
  const registerInput = (name: any, options?: any) => {
    const { ref, ...rest } = register(name, options);
    return { inputRef: ref, ...rest };
  };

  const { fields: identifierFields, append: appendIdentifier, remove: removeIdentifier } = useFieldArray({
    control,
    name: "identifiers",
  });

  const { fields: ccFields, replace: replaceCC } = useFieldArray({
    control,
    name: "custom_columns",
  });

  // Populate form with initial details
  useEffect(() => {
    if (book) {
      // Format date from "YYYY-MM-DD HH:MM:SS" to "YYYY-MM-DD"
      let formattedPubdate = "";
      if (book.pubdate) {
        const d = new Date(book.pubdate);
        if (!isNaN(d.getTime())) {
          formattedPubdate = d.toISOString().substring(0, 10);
        }
      }

      reset({
        title: book.title || "",
        authors: book.authors || "",
        series: book.series || "",
        series_index: book.series_index || "1.0",
        publisher: book.publisher || "",
        pubdate: formattedPubdate,
        rating: typeof book.rating === "number" ? book.rating : 0,
        comments: book.comments || "",
        languages: book.languages || "",
        identifiers: book.identifiers || [],
        custom_columns: book.custom_columns?.map(c => ({
          id: c.id,
          value: c.value || "",
        })) || [],
      });
    }
  }, [book, reset]);

  // Sync loaded custom columns dynamic fields
  useEffect(() => {
    if (book?.custom_columns && ccFields.length === 0) {
      replaceCC(
        book.custom_columns.map(c => ({
          id: c.id,
          value: c.value || "",
        }))
      );
    }
  }, [book, replaceCC, ccFields.length]);

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) => api(`/api/v1/book/${id}`, {
      method: "PATCH",
      body: JSON.stringify(values),
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["book", id] });
      queryClient.invalidateQueries({ queryKey: ["books"] });
      navigate(`/spa`);
    },
    onError: (err: any) => {
      setSaveError(err.message || "Failed to update book metadata.");
    },
  });

  const onSubmit = (values: FormValues) => {
    setSaveError(null);
    updateMutation.mutate(values);
  };

  if (isLoading) {
    return (
      <div style={{ display: "flex", justifyContent: "center", padding: "4rem" }}>
        <Spinner size={50} />
      </div>
    );
  }

  if (isError || !book) {
    return (
      <Callout intent="danger" title="Error Loading Book">
        Failed to fetch metadata details for book ID {id}.
      </Callout>
    );
  }

  return (
    <div style={{ maxWidth: "800px", margin: "0 auto" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "2rem" }}>
        <h3 style={{ margin: 0, fontFamily: "var(--font-serif)", textTransform: "none", fontSize: "1.8rem" }}>
          Edit Book Metadata
        </h3>
        <Button icon="arrow-left" onClick={() => navigate(-1)}>Back</Button>
      </div>

      {saveError && (
        <Callout intent="danger" title="Save Error" style={{ marginBottom: "1.5rem" }}>
          {saveError}
        </Callout>
      )}

      <form onSubmit={handleSubmit(onSubmit)}>
        <Card className="bp6-card" style={{ display: "flex", flexDirection: "column", gap: "1rem", padding: "2rem", marginBottom: "2rem" }}>
          {/* Title */}
          <FormGroup label="Book Title" labelFor="title-input">
            <InputGroup id="title-input" {...registerInput("title", { required: true })} />
          </FormGroup>

          {/* Authors */}
          <FormGroup label="Authors (separated by '&')" labelFor="authors-input">
            <InputGroup id="authors-input" {...registerInput("authors", { required: true })} />
          </FormGroup>

          {/* Series & Index */}
          <div style={{ display: "flex", gap: "1rem" }}>
            <div style={{ flex: 2 }}>
              <FormGroup label="Series" labelFor="series-input">
                <InputGroup id="series-input" {...registerInput("series")} />
              </FormGroup>
            </div>
            <div style={{ flex: 1 }}>
              <FormGroup label="Series Index" labelFor="series-index-input">
                <InputGroup id="series-index-input" {...registerInput("series_index")} />
              </FormGroup>
            </div>
          </div>

          {/* Publisher & Published Date */}
          <div style={{ display: "flex", gap: "1rem" }}>
            <div style={{ flex: 1 }}>
              <FormGroup label="Publisher" labelFor="publisher-input">
                <InputGroup id="publisher-input" {...registerInput("publisher")} />
              </FormGroup>
            </div>
            <div style={{ flex: 1 }}>
              <FormGroup label="Published Date" labelFor="pubdate-input">
                <InputGroup id="pubdate-input" type="date" {...registerInput("pubdate")} />
              </FormGroup>
            </div>
          </div>

          {/* Languages & Rating */}
          <div style={{ display: "flex", gap: "1rem" }}>
            <div style={{ flex: 1 }}>
              <FormGroup label="Languages (comma separated)" labelFor="languages-input">
                <InputGroup id="languages-input" {...registerInput("languages")} />
              </FormGroup>
            </div>
            <div style={{ flex: 1 }}>
              <FormGroup label="Rating" labelFor="rating-input">
                <HTMLSelect id="rating-input" fill {...register("rating", { valueAsNumber: true })}>
                  <option value={0}>No Rating</option>
                  <option value={1}>★</option>
                  <option value={2}>★★</option>
                  <option value={3}>★★★</option>
                  <option value={4}>★★★★</option>
                  <option value={5}>★★★★★</option>
                </HTMLSelect>
              </FormGroup>
            </div>
          </div>

          {/* Description */}
          <FormGroup label="Description / Comments" labelFor="comments-input">
            <TextArea
              id="comments-input"
              fill
              rows={8}
              style={{ fontFamily: "var(--font-serif)" }}
              {...registerInput("comments")}
            />
          </FormGroup>
        </Card>

        {/* Dynamic Identifiers Section */}
        <Card className="bp6-card" style={{ padding: "2rem", marginBottom: "2rem" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1rem" }}>
            <h6 style={{ margin: 0 }}>Identifiers</h6>
            <Button
              icon="add"
              variant="minimal"
              onClick={() => appendIdentifier({ type: "", val: "" })}
            >
              Add Identifier
            </Button>
          </div>

          {identifierFields.map((field, idx) => (
            <div key={field.id} style={{ display: "flex", gap: "1rem", alignItems: "flex-end", marginBottom: "0.5rem" }}>
              <div style={{ flex: 1 }}>
                <FormGroup label={idx === 0 ? "Type" : undefined} labelFor={`ident-type-${idx}`}>
                  <InputGroup
                    id={`ident-type-${idx}`}
                    placeholder="e.g. isbn, amazon"
                    {...registerInput(`identifiers.${idx}.type` as const, { required: true })}
                  />
                </FormGroup>
              </div>
              <div style={{ flex: 2 }}>
                <FormGroup label={idx === 0 ? "Value" : undefined} labelFor={`ident-val-${idx}`}>
                  <InputGroup
                    id={`ident-val-${idx}`}
                    placeholder="Value"
                    {...registerInput(`identifiers.${idx}.val` as const, { required: true })}
                  />
                </FormGroup>
              </div>
              <Button
                icon="trash"
                intent="danger"
                variant="minimal"
                onClick={() => removeIdentifier(idx)}
                style={{ marginBottom: "15px" }}
              />
            </div>
          ))}
        </Card>

        {/* Custom Columns Section */}
        {book.custom_columns && book.custom_columns.length > 0 && (
          <Card className="bp6-card" style={{ padding: "2rem", marginBottom: "2rem" }}>
            <h6 style={{ marginBottom: "1rem" }}>Custom Columns</h6>
            {ccFields.map((field, idx) => {
              const def = book.custom_columns?.find(c => c.id === field.id);
              if (!def) return null;

              return (
                <div key={field.id} style={{ marginBottom: "1rem" }}>
                  {def.datatype === "bool" ? (
                    <FormGroup>
                      <Switch
                        label={def.name}
                        checked={watch(`custom_columns.${idx}.value` as any) === "True"}
                        onChange={(e) => {
                          setValue(`custom_columns.${idx}.value` as any, e.target.checked ? "True" : "False");
                        }}
                        large
                      />
                    </FormGroup>
                  ) : def.datatype === "comments" ? (
                    <FormGroup label={def.name} labelFor={`cc-val-${idx}`}>
                      <TextArea
                        id={`cc-val-${idx}`}
                        fill
                        rows={4}
                        {...registerInput(`custom_columns.${idx}.value` as const)}
                      />
                    </FormGroup>
                  ) : (
                    <FormGroup label={def.name} labelFor={`cc-val-${idx}`}>
                      <InputGroup
                        id={`cc-val-${idx}`}
                        {...registerInput(`custom_columns.${idx}.value` as const)}
                      />
                    </FormGroup>
                  )}
                </div>
              );
            })}
          </Card>
        )}

        {/* Action Buttons */}
        <div style={{ display: "flex", justifyContent: "flex-end", gap: "1rem" }}>
          <Button
            large
            onClick={() => navigate(-1)}
          >
            Cancel
          </Button>
          <Button
            large
            intent="primary"
            type="submit"
            loading={updateMutation.isPending}
          >
            Save Changes
          </Button>
        </div>
      </form>
    </div>
  );
}
