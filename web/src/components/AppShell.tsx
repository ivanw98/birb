import type { ReactNode } from "react";
import type { View } from "../App";
import { useLiveQuery } from "dexie-react-hooks";
import { db } from "@/db/db";

interface AppShellProps {
  view: View;
  onNavigate: (view: View) => void;
  headerRight?: ReactNode;
  children: ReactNode;
}

type NavItemProps = {
  view: View;
  label: string;
};
const NAV_ITEMS: Array<NavItemProps> = [
  { view: "record", label: "Record" },
  { view: "sightings", label: "Sightings" },
];

export function AppShell({
  view,
  onNavigate,
  headerRight,
  children,
}: AppShellProps) {
  const querier = () =>
    db.sightings.where("syncStatus").equals("pending").count();
  const pendingCount = useLiveQuery(querier);
  const showPendingCount = pendingCount !== undefined && pendingCount !== 0;
  return (
    <div className="flex min-h-screen flex-col bg-white text-ink">
      <a
        href="#main-content"
        className="sr-only focus-visible:not-sr-only focus-visible:fixed focus-visible:top-2 focus-visible:left-2 focus-visible:z-50 focus-visible:rounded-md focus-visible:bg-primary focus-visible:px-4 focus-visible:py-2 focus-visible:text-white"
      >
        Skip to main content
      </a>
      <header className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
        <h1 className="text-xl font-semibold">birb</h1>
        {headerRight}
      </header>

      <main id="main-content" className="flex-1 pb-24">
        {children}
      </main>

      <nav
        aria-label="Main"
        className="sticky bottom-0 flex border-t border-slate-200 bg-white"
      >
        {NAV_ITEMS.map((item) => {
          const isActive = view === item.view;
          return (
            <button
              key={item.view}
              type="button"
              aria-current={isActive ? "page" : undefined}
              onClick={() => onNavigate(item.view)}
              className={`min-h-12 flex-1 border-t-2 px-4 py-3 text-lg ${
                isActive
                  ? "border-primary font-semibold text-primary"
                  : "border-transparent font-medium text-muted"
              }`}
            >
              {item.label}
              {item.view === "sightings" &&
                showPendingCount &&
                ` (${pendingCount})`}
            </button>
          );
        })}
      </nav>
    </div>
  );
}
