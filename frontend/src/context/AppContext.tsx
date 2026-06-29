import React, { createContext, useContext, useEffect, useState } from "react";
import { api } from "../api/client";

export interface UserSession {
  id: number;
  nickname: string;
  email: string;
  role_admin: boolean;
  role_edit: boolean;
  role_download: boolean;
  role_upload: boolean;
  role_delete_books: boolean;
  locale: string;
}

export interface AppConfig {
  books_per_page: number;
  authors_max: number;
  upload_enabled: boolean;
  kobo_enabled: boolean;
  [key: string]: any;
}

interface AppContextType {
  user: UserSession | null;
  config: AppConfig | null;
  locale: string;
  csrfToken: string | null;
  loading: boolean;
  t: (key: string, defaultText?: string) => string;
  refetchSession: () => Promise<void>;
}

const AppContext = createContext<AppContextType | undefined>(undefined);

export function AppProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<UserSession | null>(null);
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [locale, setLocale] = useState<string>("en");
  const [csrfToken, setCsrfToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchSession = async () => {
    try {
      setLoading(true);
      const data = await api<{
        user: UserSession | null;
        config: AppConfig;
        csrf_token: string;
        locale: string;
      }>("/api/v1/session");

      setUser(data.user);
      setConfig(data.config);
      setLocale(data.locale || "en");
      setCsrfToken(data.csrf_token);
      if (data.csrf_token) {
        (window as any).__CSRF__ = data.csrf_token;
      }
    } catch (err) {
      console.error("Failed to load session", err);
      // Fallback defaults for standalone/dev frontend runs without backend up
      setUser({
        id: 1,
        nickname: "Guest Admin",
        email: "admin@example.com",
        role_admin: true,
        role_edit: true,
        role_download: true,
        role_upload: true,
        role_delete_books: true,
        locale: "en"
      });
      setConfig({
        books_per_page: 60,
        authors_max: 5,
        upload_enabled: true,
        kobo_enabled: false
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSession();
  }, []);

  // Simple i18n translation lookup wrapper
  const t = (key: string, defaultText?: string) => {
    // For now, return defaultText or key. Can hook into translation bundles here.
    return defaultText || key;
  };

  return (
    <AppContext.Provider
      value={{
        user,
        config,
        locale,
        csrfToken,
        loading,
        t,
        refetchSession: fetchSession
      }}
    >
      {children}
    </AppContext.Provider>
  );
}

export function useApp() {
  const context = useContext(AppContext);
  if (!context) {
    throw new Error("useApp must be used within an AppProvider");
  }
  return context;
}
