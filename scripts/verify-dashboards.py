#!/usr/bin/env python3
"""Mechanically verify that every Grafana dashboard panel has data for every
successful run topology.

For each distinct completed topology (db kind + node count) it picks the latest
COMPLETED run, then for each dashboard surfaced for that run (workload + system +
the db-kind dashboard) it extracts every panel's PromQL, substitutes the
dashboard template variables for the run, and range-queries VictoriaMetrics over
the run's [startedAt, finishedAt] window. A panel "has data" if any series
returns a finite value in the window.

Run on the stroppy-cloud host (needs the postgres container, vmauth, and the
dashboards dir). Env: MONITORING_TOKEN, optional REPO_DIR.

Output: per run/dashboard panel coverage, and the list of empty/errored panels
so legitimate not-applicable panels (e.g. swap on diskless VMs, replication on a
single node) can be told apart from real gaps.
"""
import subprocess, json, os, glob, urllib.parse, urllib.request, urllib.error, re
from datetime import datetime

REPO = os.environ.get("REPO_DIR", "/home/st-postgres/stroppy-cloud")
DASH_DIR = REPO + "/deployments/grafana/dashboards"
VM = "http://localhost:8427/select/multitenant/prometheus/api/v1/query_range"
MT = os.environ.get("MONITORING_TOKEN", "stroppy-monitoring-secret")
KIND_DASH = {
    "KIND_POSTGRES": "stroppy-postgres", "KIND_MYSQL": "stroppy-mysql",
    "KIND_MARIADB": "stroppy-mysql", "KIND_COCKROACH": "stroppy-cockroach",
    "KIND_YDB": "stroppy-ydb", "KIND_PICODATA": "stroppy-picodata",
}

UID_FILE = {}
for f in glob.glob(DASH_DIR + "/*.json"):
    try:
        UID_FILE[json.load(open(f)).get("uid")] = f
    except Exception:
        pass


def psql(sql):
    r = subprocess.run(
        ["docker", "compose", "exec", "-T", "postgres", "psql", "-U", "stroppy",
         "-d", "stroppy", "-At", "-F", "|", "-c", sql],
        cwd=REPO, capture_output=True, text=True)
    if r.returncode != 0:
        print("PSQL ERR:", r.stderr[:200])
    return [l for l in r.stdout.splitlines() if l.strip()]


def ts(s):
    try:
        return datetime.fromisoformat(s.replace("Z", "+00:00")).timestamp()
    except Exception:
        return None


def subst(expr, runid, win):
    """Substitute dashboard template vars for a concrete run."""
    pfx = "stroppy_" + runid.replace("-", "_") + "_"
    expr = expr.replace("${prefix}", pfx).replace("$prefix", pfx)
    expr = expr.replace("${run_id}", runid).replace("$run_id", runid)
    expr = re.sub(r"\$__range_s\b", str(int(win)), expr)
    expr = re.sub(r"\$__range\b", f"{int(win)}s", expr)
    expr = re.sub(r"\$(__rate_interval|__interval|Interval)\b", "1m", expr)
    # A var used as a label match MUST stay a regex: label="$v" -> label=~".+"
    expr = re.sub(r'=\s*"\$\{?\w+\}?"', '=~".+"', expr)
    expr = re.sub(r'=~\s*"\$\{?\w+\}?"', '=~".+"', expr)
    # Any remaining bare var -> .+
    expr = re.sub(r"\$\{(\w+)\}", ".+", expr)
    expr = re.sub(r"\$(\w+)", ".+", expr)
    return expr


def panels(dash):
    out = []

    def walk(ps):
        for p in ps or []:
            if p.get("type") == "row":
                walk(p.get("panels"))
                continue
            for t in p.get("targets", []) or []:
                e = t.get("expr")
                if e and e.strip():
                    out.append((p.get("title", "?"), e))
            if p.get("panels"):
                walk(p.get("panels"))

    walk(dash.get("panels"))
    return out


def has_data(expr, start, end):
    q = urllib.parse.urlencode({"query": expr, "start": int(start), "end": int(end), "step": "30"})
    try:
        d = json.load(urllib.request.urlopen(
            urllib.request.Request(VM + "?" + q, headers={"Authorization": "Bearer " + MT}), timeout=25))
    except urllib.error.HTTPError as ex:
        return None, f"HTTP{ex.code}"
    except Exception as ex:
        return None, str(ex)[:40]
    res = d.get("data", {}).get("result", [])
    if not res:
        return False, "no series"
    for s in res:
        for _, v in s.get("values", []):
            if v not in ("NaN", "+Inf", "-Inf"):
                return True, ""
    return False, "all NaN"


def main():
    raw = psql(
        "SELECT data->'summary'->>'dbKind', coalesce(data->'summary'->>'nodeCount','?'), id, "
        "data->'summary'->>'startedAt', data->'summary'->>'finishedAt' "
        "FROM test_run_records WHERE data->>'status'='STATUS_COMPLETED' ORDER BY created_at DESC")
    seen, rows = set(), []
    for l in raw:
        k = tuple(l.split("|")[:2])
        if k in seen:
            continue
        seen.add(k)
        rows.append(l)
    print(f"completed runs: {len(raw)}, distinct topologies (kind+nodeCount): {len(rows)}\n")
    tp = te = 0
    for l in rows:
        kind, nc, rid, st, fin = (l.split("|") + [""] * 5)[:5]
        s, e = ts(st), ts(fin)
        if not s or not e or e <= s:
            print(f"## {kind} nodes={nc} run={rid[:8]} BAD-WINDOW")
            continue
        win = e - s
        dashes = ["stroppy-metrics-v1", "stroppy-system"] + ([KIND_DASH[kind]] if kind in KIND_DASH else [])
        print(f"## {kind}  nodes={nc}  run={rid[:8]}  win={int(win)}s")
        for du in dashes:
            f = UID_FILE.get(du)
            if not f:
                print(f"   [{du}] MISSING FILE")
                continue
            ps = panels(json.load(open(f)))
            empty = []
            for title, expr in ps:
                ok, why = has_data(subst(expr, rid, win), s, e)
                tp += 1
                if ok is not True:
                    empty.append((title, why))
                    te += 1
            print(f"   [{du}] {len(ps)} panels -> {'OK' if not empty else str(len(empty)) + ' EMPTY/ERR'}")
            for title, why in empty[:30]:
                print(f"        - {title} [{why}]")
            if len(empty) > 30:
                print(f"        ... +{len(empty) - 30} more")
    print(f"\n=== TOTAL {tp} panel-queries, {te} empty/err ===")


if __name__ == "__main__":
    main()
