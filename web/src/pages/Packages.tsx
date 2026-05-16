import { useEffect, useState, useCallback } from "react";
import { clients } from "@/api/clients";
import { getAccessToken } from "@/api/transport";
import { Database_Kind } from "@/lib/proto/cloud/v1/catalog/database_pb";
import {
  type Package as ProtoPackage,
} from "@/lib/proto/cloud/v1/catalog/package_pb";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Plus,
  Trash2,
  Copy,
  Upload,
  X,
  Check,
  AlertCircle,
  Database,
  Server,
  Cpu,
  Package as PackageIcon,
  Pencil,
  FileDown,
} from "lucide-react";

// ─── Local UI type ────────────────────────────────────────────────

interface PkgRow {
  id: string;
  name: string;
  description: string;
  db_kind: string; // human-readable string
  db_kind_proto: Database_Kind;
  db_version: string;
  is_builtin: boolean;
  apt_packages: string[];
  pre_install: string[];
  custom_repo: string;
  custom_repo_key: string;
  deb_filename: string;
  has_deb: boolean;
}

// ─── Enum helpers ─────────────────────────────────────────────────

const KIND_TO_STRING: Record<Database_Kind, string> = {
  [Database_Kind.DATABASE_KIND_UNSPECIFIED]: "",
  [Database_Kind.DATABASE_KIND_POSTGRES]: "postgres",
  [Database_Kind.DATABASE_KIND_MYSQL]: "mysql",
  [Database_Kind.DATABASE_KIND_MARIADB]: "mariadb",
  [Database_Kind.DATABASE_KIND_YDB]: "ydb",
  [Database_Kind.DATABASE_KIND_YDB_MANAGED]: "ydb-managed",
  [Database_Kind.DATABASE_KIND_COCKROACH]: "cockroach",
  [Database_Kind.DATABASE_KIND_PICODATA]: "picodata",
};

const STRING_TO_KIND: Record<string, Database_Kind> = {
  postgres: Database_Kind.DATABASE_KIND_POSTGRES,
  mysql: Database_Kind.DATABASE_KIND_MYSQL,
  mariadb: Database_Kind.DATABASE_KIND_MARIADB,
  ydb: Database_Kind.DATABASE_KIND_YDB,
  "ydb-managed": Database_Kind.DATABASE_KIND_YDB_MANAGED,
  cockroach: Database_Kind.DATABASE_KIND_COCKROACH,
  picodata: Database_Kind.DATABASE_KIND_PICODATA,
};

function protoToRow(p: ProtoPackage): PkgRow {
  const apt = p.source?.source?.case === "apt" ? p.source.source.value : null;
  const deb = p.source?.source?.case === "debBlob" ? p.source.source.value : null;
  return {
    id: p.id?.value ?? "",
    name: p.identity?.name ?? "",
    description: p.identity?.description ?? "",
    db_kind: KIND_TO_STRING[p.dbKind] ?? "",
    db_kind_proto: p.dbKind,
    db_version: p.dbVersion,
    is_builtin: p.isBuiltin,
    apt_packages: apt?.aptPackages ?? [],
    pre_install: apt?.preInstall ?? [],
    custom_repo: apt?.customRepo ?? "",
    custom_repo_key: apt?.customRepoKey ?? "",
    deb_filename: deb?.debFilename ?? "",
    has_deb: !!deb,
  };
}

async function uploadPackageDeb(packageId: string, file: File): Promise<void> {
  const formData = new FormData();
  formData.append("file", file);
  const headers: Record<string, string> = {};
  const token = getAccessToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  const res = await fetch(`/api/v1/packages/${packageId}/deb`, {
    method: "POST",
    headers,
    body: formData,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText);
    throw new Error(text || `HTTP ${res.status}`);
  }
}

// ─── Icons ────────────────────────────────────────────────────────

const DB_ICONS: Record<string, typeof Database> = {
  postgres: Database,
  mysql: Server,
  picodata: Cpu,
};

// ─── Main page ───────────────────────────────────────────────────

export function Packages() {
  const [packages, setPackages] = useState<PkgRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState("");
  const [editing, setEditing] = useState<PkgRow | null>(null);
  const [creating, setCreating] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  const confirm = useConfirm();

  const load = useCallback(async () => {
    try {
      const resp = await clients.package.listPackages({});
      setPackages((resp.packages ?? []).map(protoToRow));
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed to load" });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const filtered = packages.filter((p) =>
    !filter || p.name.toLowerCase().includes(filter.toLowerCase()) || p.db_kind.includes(filter)
  );

  async function handleDelete(id: string) {
    if (!(await confirm({ title: "Delete this package?", description: "This action cannot be undone.", danger: true }))) return;
    try {
      await clients.package.deletePackage({ value: id });
      setMessage({ type: "success", text: "Deleted" });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  async function handleClone(id: string) {
    try {
      const r = await clients.package.clonePackage({ value: id });
      setMessage({ type: "success", text: `Cloned as "${r.identity?.name}"` });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Packages</h1>
          <p className="text-sm text-muted-foreground">Database packages for runs</p>
        </div>
        <Button size="sm" onClick={() => setCreating(true)}>
          <Plus className="h-3.5 w-3.5" /> New Package
        </Button>
      </div>

      {message && (
        <div className={`flex items-center gap-2 text-xs p-2 border font-mono ${
          message.type === "success" ? "border-success/30 text-success" : "border-destructive/30 text-destructive"
        }`}>
          {message.type === "success" ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
          {message.text}
        </div>
      )}

      <Input
        placeholder="Filter by name or db kind..."
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className="max-w-sm h-8 text-xs font-mono"
      />

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading...</p>
      ) : (
        <div className="border border-zinc-800">
          {/* Header */}
          <div className="grid grid-cols-[1fr_100px_60px_80px_60px_120px] gap-2 px-4 py-2 border-b border-zinc-800 text-[10px] text-zinc-500 uppercase tracking-wider font-mono">
            <span>Name</span>
            <span>DB</span>
            <span>Ver</span>
            <span>Packages</span>
            <span>.deb</span>
            <span>Actions</span>
          </div>

          {filtered.length === 0 && (
            <div className="px-4 py-6 text-sm text-zinc-600 text-center">No packages found</div>
          )}

          {filtered.map((pkg) => {
            const Icon = DB_ICONS[pkg.db_kind] || PackageIcon;
            return (
              <div key={pkg.id} className="grid grid-cols-[1fr_100px_60px_80px_60px_120px] gap-2 px-4 py-2 border-b border-zinc-800/50 items-center hover:bg-zinc-900/30">
                <div className="flex items-center gap-2 min-w-0">
                  <Icon className="w-3.5 h-3.5 text-zinc-500 shrink-0" />
                  <span className="text-xs font-mono text-zinc-200 truncate">{pkg.name}</span>
                  {pkg.is_builtin && <Badge variant="secondary" className="text-[8px] shrink-0">builtin</Badge>}
                </div>
                <span className="text-xs font-mono text-zinc-400">{pkg.db_kind}</span>
                <span className="text-xs font-mono text-zinc-500">{pkg.db_version}</span>
                <span className="text-[10px] font-mono text-zinc-500">{pkg.apt_packages.length} apt</span>
                <span className="text-[10px] font-mono text-zinc-500">{pkg.has_deb ? "yes" : "—"}</span>
                <div className="flex items-center gap-1">
                  <button onClick={() => setEditing(pkg)} className="p-1 text-zinc-600 hover:text-zinc-300" title="Edit">
                    <Pencil className="w-3 h-3" />
                  </button>
                  <button onClick={() => handleClone(pkg.id)} className="p-1 text-zinc-600 hover:text-zinc-300" title="Clone">
                    <Copy className="w-3 h-3" />
                  </button>
                  {!pkg.is_builtin && (
                    <button onClick={() => handleDelete(pkg.id)} className="p-1 text-zinc-600 hover:text-red-400" title="Delete">
                      <Trash2 className="w-3 h-3" />
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Create/Edit modal */}
      {(creating || editing) && (
        <PackageEditor
          pkg={editing}
          onClose={() => { setEditing(null); setCreating(false); }}
          onSaved={() => { setEditing(null); setCreating(false); load(); }}
        />
      )}
    </div>
  );
}

// ─── Package editor (modal-like overlay) ─────────────────────────

function PackageEditor({
  pkg,
  onClose,
  onSaved,
}: {
  pkg: PkgRow | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const isEdit = !!pkg;
  const [name, setName] = useState(pkg?.name || "");
  const [description, setDescription] = useState(pkg?.description || "");
  const [dbKind, setDbKind] = useState(pkg?.db_kind || "postgres");
  const [dbVersion, setDbVersion] = useState(pkg?.db_version || "");
  const [aptPackages, setAptPackages] = useState<string[]>(pkg?.apt_packages || []);
  const [preInstall, setPreInstall] = useState<string[]>(pkg?.pre_install || []);
  const [customRepo, setCustomRepo] = useState(pkg?.custom_repo || "");
  const [customRepoKey, setCustomRepoKey] = useState(pkg?.custom_repo_key || "");
  const [aptDraft, setAptDraft] = useState("");
  const [preDraft, setPreDraft] = useState("");
  const [debFile, setDebFile] = useState<File | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  async function handleSave() {
    if (!name || !dbKind) { setError("Name and DB kind required"); return; }
    setSaving(true);
    setError("");
    try {
      const pkgProto = {
        identity: { name, description },
        dbKind: STRING_TO_KIND[dbKind] ?? Database_Kind.DATABASE_KIND_UNSPECIFIED,
        dbVersion,
        source: {
          source: {
            case: "apt" as const,
            value: {
              aptPackages,
              preInstall,
              customRepo,
              customRepoKey,
            },
          },
        },
      };

      let targetId = pkg?.id;
      if (isEdit) {
        await clients.package.updatePackage({
          package: { id: { value: pkg!.id }, ...pkgProto },
          updateMask: { paths: ["identity", "db_version", "source"] },
        });
      } else {
        const res = await clients.package.createPackage({ package: pkgProto });
        targetId = res.id?.value;
      }

      if (debFile && targetId) {
        await uploadPackageDeb(targetId, debFile);
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    }
    setSaving(false);
  }

  function addApt() {
    const v = aptDraft.trim();
    if (v && !aptPackages.includes(v)) { setAptPackages([...aptPackages, v]); setAptDraft(""); }
  }

  function addPre() {
    const v = preDraft.trim();
    if (v) { setPreInstall([...preInstall, v]); setPreDraft(""); }
  }

  return (
    <div className="fixed inset-0 bg-black/60 z-50 flex items-center justify-center p-4" onClick={onClose}>
      <div className="bg-[#0a0a0a] border border-zinc-800 w-full max-w-2xl max-h-[80vh] overflow-y-auto p-5 space-y-4" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold">{isEdit ? "Edit Package" : "New Package"}</h2>
          <button onClick={onClose} className="text-zinc-600 hover:text-zinc-300"><X className="w-4 h-4" /></button>
        </div>

        {error && (
          <div className="text-xs text-destructive border border-destructive/30 p-2 font-mono">{error}</div>
        )}

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} className="h-8 text-xs font-mono" placeholder="OrioleDB 16" disabled={pkg?.is_builtin} />
          </div>
          <div className="space-y-1.5">
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Description</Label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} className="h-8 text-xs font-mono" placeholder="Custom build..." disabled={pkg?.is_builtin} />
          </div>
          <div className="space-y-1.5">
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">DB Kind</Label>
            <Select value={dbKind} onValueChange={setDbKind} disabled={pkg?.is_builtin}>
              <SelectTrigger className="h-8 font-mono text-xs"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="postgres">postgres</SelectItem>
                <SelectItem value="mysql">mysql</SelectItem>
                <SelectItem value="picodata">picodata</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">DB Version</Label>
            <Input value={dbVersion} onChange={(e) => setDbVersion(e.target.value)} className="h-8 text-xs font-mono" placeholder="16" disabled={pkg?.is_builtin} />
          </div>
        </div>

        {/* APT packages */}
        <div className="space-y-1.5">
          <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">APT Packages</Label>
          {aptPackages.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {aptPackages.map((v, i) => (
                <span key={i} className="inline-flex items-center gap-1 text-[11px] font-mono px-2 py-0.5 border border-zinc-700 text-zinc-300 bg-zinc-900/50">
                  {v}
                  {!pkg?.is_builtin && (
                    <button onClick={() => setAptPackages(aptPackages.filter((_, j) => j !== i))} className="text-zinc-600 hover:text-red-400">
                      <X className="w-2.5 h-2.5" />
                    </button>
                  )}
                </span>
              ))}
            </div>
          )}
          {!pkg?.is_builtin && (
            <div className="flex gap-1.5">
              <Input value={aptDraft} onChange={(e) => setAptDraft(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addApt(); } }} placeholder="package name" className="h-7 text-xs font-mono" />
              <Button variant="outline" size="sm" onClick={addApt} className="h-7 px-2"><Plus className="w-3 h-3" /></Button>
            </div>
          )}
        </div>

        {/* Pre-install commands */}
        <div className="space-y-1.5">
          <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Pre-install Commands</Label>
          {preInstall.length > 0 && (
            <div className="space-y-1">
              {preInstall.map((v, i) => (
                <div key={i} className="flex items-center gap-1 group">
                  <span className="flex-1 text-[10px] font-mono text-zinc-400 bg-zinc-900/50 border border-zinc-800/50 px-2 py-1 truncate">{v}</span>
                  {!pkg?.is_builtin && (
                    <button onClick={() => setPreInstall(preInstall.filter((_, j) => j !== i))} className="text-zinc-700 hover:text-red-400 opacity-0 group-hover:opacity-100">
                      <X className="w-3 h-3" />
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
          {!pkg?.is_builtin && (
            <div className="flex gap-1.5">
              <Input value={preDraft} onChange={(e) => setPreDraft(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addPre(); } }} placeholder="shell command" className="h-7 text-xs font-mono" />
              <Button variant="outline" size="sm" onClick={addPre} className="h-7 px-2"><Plus className="w-3 h-3" /></Button>
            </div>
          )}
        </div>

        {/* Custom repo */}
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Custom APT Repo</Label>
            <Input value={customRepo} onChange={(e) => setCustomRepo(e.target.value)} className="h-7 text-xs font-mono" placeholder="deb https://..." disabled={pkg?.is_builtin} />
          </div>
          <div className="space-y-1.5">
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">GPG Key URL</Label>
            <Input value={customRepoKey} onChange={(e) => setCustomRepoKey(e.target.value)} className="h-7 text-xs font-mono" placeholder="https://..." disabled={pkg?.is_builtin} />
          </div>
        </div>

        {/* .deb file */}
        {!pkg?.is_builtin && (
          <div className="border border-dashed border-zinc-800 p-3 space-y-2">
            <div className="flex items-center gap-2">
              <FileDown className="w-3.5 h-3.5 text-zinc-500" />
              <span className="text-[11px] font-mono text-zinc-400 uppercase tracking-wider">.deb File</span>
              {pkg?.deb_filename && <span className="text-[10px] font-mono text-zinc-600">current: {pkg.deb_filename}</span>}
            </div>
            <label className="cursor-pointer inline-block">
              <input type="file" accept=".deb" onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) setDebFile(f);
              }} className="hidden" />
              <Button asChild variant="outline" size="sm">
                <span><Upload className="h-3 w-3" />{debFile ? debFile.name : "Choose .deb"}</span>
              </Button>
            </label>
            {debFile && <div className="text-[10px] font-mono text-zinc-500">{debFile.name}</div>}
            <p className="text-[9px] text-zinc-700 font-mono">File will be uploaded on save</p>
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center justify-end gap-2 pt-2 border-t border-zinc-800">
          <Button variant="outline" size="sm" onClick={onClose}>Cancel</Button>
          {!pkg?.is_builtin && (
            <Button size="sm" onClick={handleSave} disabled={saving}>
              {saving ? "Saving..." : isEdit ? "Save" : "Create"}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
