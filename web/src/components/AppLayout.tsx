import { Outlet } from "react-router-dom";
import { Header } from "@/components/header/Header";
import { TenantSidebar } from "@/components/TenantSidebar";

// Tenant-scoped shell: top header + per-tenant sidebar. Used for /t/:slug/*.
export function AppLayout() {
  return (
    <div className="flex h-screen flex-col overflow-hidden">
      <Header />
      <div className="flex flex-1 overflow-hidden">
        <TenantSidebar />
        <main className="flex-1 overflow-auto">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
