import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App.tsx";
import { AuthProvider } from "./auth/AuthProvider.tsx";
import { ensurePersistence } from "./lib/storage.ts";
import { supabaseBackend } from "./auth/supabaseBackend.ts";

void ensurePersistence();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AuthProvider backend={supabaseBackend}>
      <App />
    </AuthProvider>
  </StrictMode>,
);
