import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Play, Database, Activity, Zap } from "lucide-react";

export function Dashboard() {
  const [healthy, setHealthy] = useState<boolean | null>(null);

  // Health check via direct fetch — no legacy client dependency.
  useEffect(() => {
    let cancelled = false;
    async function check() {
      try {
        const res = await fetch("/health");
        if (!cancelled) setHealthy(res.ok);
      } catch {
        if (!cancelled) setHealthy(false);
      }
    }
    check();
    const interval = setInterval(check, 10000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  // Update health indicator in sidebar
  useEffect(() => {
    const dot = document.getElementById("health-indicator");
    const text = document.getElementById("health-text");
    if (dot) {
      dot.className = `w-2 h-2 ${
        healthy === null
          ? "bg-warning"
          : healthy
            ? "bg-success"
            : "bg-destructive"
      }`;
    }
    if (text) {
      text.textContent = healthy === null
        ? "Checking..."
        : healthy
          ? "Connected"
          : "Offline";
    }
  }, [healthy]);

  const quickStarts = [
    {
      label: "Postgres Single",
      kind: "postgres",
      preset: "single",
      icon: Database,
    },
    { label: "Postgres HA", kind: "postgres", preset: "ha", icon: Database },
    { label: "MySQL Single", kind: "mysql", preset: "single", icon: Database },
    { label: "MySQL Group", kind: "mysql", preset: "group", icon: Database },
    {
      label: "Picodata Cluster",
      kind: "picodata",
      preset: "cluster",
      icon: Zap,
    },
  ];

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Dashboard</h1>
          <p className="text-sm text-muted-foreground">
            Monitor active runs and launch new tests
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant={healthy ? "success" : healthy === false ? "destructive" : "warning"}>
            <Activity className="h-3 w-3 mr-1" />
            {healthy === null ? "checking" : healthy ? "healthy" : "offline"}
          </Badge>
          <Link to="/runs/new">
            <Button size="sm">
              <Play className="h-3.5 w-3.5" />
              New Run
            </Button>
          </Link>
        </div>
      </div>

      {/* Quick start */}
      <div>
        <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">
          Quick Start
        </h2>
        <div className="grid grid-cols-5 gap-3">
          {quickStarts.map((qs) => (
            <Link
              key={qs.label}
              to={`/runs/new?kind=${qs.kind}&preset=${qs.preset}`}
            >
              <Card className="hover:border-primary/50 transition-colors cursor-pointer">
                <CardContent className="p-3 flex items-center gap-2">
                  <qs.icon className="h-4 w-4 text-primary" />
                  <span className="text-xs font-medium">{qs.label}</span>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      </div>

      {/* Active runs — link to runs page for live view */}
      <div>
        <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">
          Active Runs
        </h2>
        <Card>
          <CardContent className="p-8 text-center">
            <p className="text-sm text-muted-foreground">
              View all runs and their live status on the{" "}
              <Link to="/runs" className="text-primary hover:underline">
                Test Runs
              </Link>{" "}
              page.
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
