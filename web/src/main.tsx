import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";

import { App } from "@/App";
import { ConfirmProvider } from "@/components/confirm-dialog";
import { ErrorBoundary } from "@/components/error-boundary";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AuthProvider } from "@/contexts/auth-context";
import "@/index.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <BrowserRouter>
        <AuthProvider>
          <TooltipProvider delayDuration={200}>
            <ConfirmProvider>
              <App />
            </ConfirmProvider>
          </TooltipProvider>
        </AuthProvider>
        <Toaster richColors position="top-right" />
      </BrowserRouter>
    </ErrorBoundary>
  </StrictMode>,
);
