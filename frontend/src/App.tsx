import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { BlueprintProvider, Spinner, SpinnerSize, NonIdealState } from "@blueprintjs/core";
import { AppProvider, useApp } from "./context/AppContext";
import { Layout } from "./components/Layout";
import BooksPage from "./routes/books/BooksPage";
import BookEditPage from "./routes/edit/BookEditPage";
import BookReaderPage from "./routes/read/BookReaderPage";
import { LoginPage } from "./routes/auth/LoginPage";
import { RegisterPage } from "./routes/auth/RegisterPage";

import ConfigPage from "./routes/config/ConfigPage";
import AdminPage from "./routes/admin/AdminPage";
import DatasetsPage from "./routes/datasets/DatasetsPage";
import DatasetDetailPage from "./routes/datasets/DatasetDetailPage";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});

function AppRoutes() {
  const { user, config, loading } = useApp();

  if (loading) {
    return (
      <div style={{ display: "flex", justifyContent: "center", alignItems: "center", height: "100vh", backgroundColor: "var(--bg-primary)" }}>
        <Spinner size={SpinnerSize.LARGE} intent="primary" />
      </div>
    );
  }

  const isGuest = !user;
  const anonBrowse = config?.anonymous_browse;

  // If anonymous browsing is disabled and user is not logged in, enforce login/register routes
  if (isGuest && !anonBrowse) {
    return (
      <Routes>
        <Route path="/spa/register" element={<RegisterPage />} />
        <Route path="*" element={<LoginPage />} />
      </Routes>
    );
  }

  return (
    <Routes>
      {/* Redirect base / to /spa */}
      <Route path="/" element={<Navigate to="/spa" replace />} />
      
      {/* SPA routes */}
      <Route path="/spa" element={<Layout />}>
        {/* Books Grid */}
        <Route index element={<BooksPage />} />
        
        {/* Search */}
        <Route path="search" element={<NonIdealState icon="search" title="Search Books" />} />
        
        {/* Secondary Browsing Pages */}
        <Route path="authors" element={<NonIdealState icon="people" title="Authors" description="Author directory" />} />
        <Route path="series" element={<NonIdealState icon="properties" title="Series" description="Book series catalog" />} />
        <Route path="publishers" element={<NonIdealState icon="office" title="Publishers" description="Publisher directory" />} />
        <Route path="categories" element={<NonIdealState icon="tag" title="Categories" description="Book genres and categories" />} />
        
        {/* Shelves */}
        <Route path="shelves" element={<NonIdealState icon="star" title="Shelves" description="Your bookshelves" />} />
        
        {/* Configuration / Admin */}
        <Route path="config" element={<ConfigPage />} />
        <Route path="admin" element={<AdminPage />} />

        {/* Datasets */}
        <Route path="datasets" element={<DatasetsPage />} />
        <Route path="datasets/:id" element={<DatasetDetailPage />} />

        {/* Book Edit */}
        <Route path="edit/:id" element={<BookEditPage />} />

        {/* Guest auth routes in case anonBrowse is enabled */}
        {isGuest && (
          <>
            <Route path="login" element={<LoginPage />} />
            <Route path="register" element={<RegisterPage />} />
          </>
        )}

        {/* 404 Route */}
        <Route path="*" element={<NonIdealState icon="warning-sign" title="Not Found" description="The page you requested does not exist." />} />
      </Route>

      {/* Reader Page outside Layout */}
      <Route path="/spa/read/:id/:format" element={<BookReaderPage />} />
      
      {/* 404 fallback */}
      <Route path="*" element={<Navigate to="/spa" replace />} />
    </Routes>
  );
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppProvider>
          <BlueprintProvider>
            <AppRoutes />
          </BlueprintProvider>
        </AppProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

