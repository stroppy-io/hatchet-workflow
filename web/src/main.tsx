import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { AuthProvider } from "@/contexts/AuthContext";
import { ConfirmProvider } from "@/components/ui/confirm-dialog";
import "./index.css";

function boot() {
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <BrowserRouter>
        <AuthProvider>
          <ConfirmProvider>
            <App />
          </ConfirmProvider>
        </AuthProvider>
      </BrowserRouter>
    </StrictMode>,
  );
}

// <<< MOCK GATE (single flag). To remove the throwaway mock entirely:
//     1. delete this if/else and call boot() directly, and
//     2. delete the src/mock/ directory.
//     Nothing else in the app references the mock. >>>
if (import.meta.env.VITE_MOCK === "1") {
  import("@/mock").then(({ installMock }) => {
    installMock();
    boot();
  });
} else {
  boot();
}
// <<< END MOCK GATE >>>
