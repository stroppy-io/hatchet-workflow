import { Outlet } from "react-router-dom";
import { Header } from "@/components/header/Header";

// Outside-tenant shell: top header only, no tenant sidebar. Used for the
// platform-admin area (/admin/*) and the user profile (/profile).
export function PlainLayout() {
  return (
    <div className="flex h-screen flex-col overflow-hidden">
      <Header />
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
