import { useEffect, useState, useCallback } from "react";
import {
  listPackages,
  createPackage,
  deletePackage,
  clonePackage,
  updatePackage,
  uploadPackageDeb,
} from "@/api/client";
import type { Package } from "@/api/types";
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

const DB_ICONS: Record<string, typeof Database> = {
  postgres: Database,
  mysql: Server,
  picodata: Cpu,
};

export function Packages() {
  const [packages, setPackages] = useState<Package[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState("");
  const [editing, setEditing] = useState<Package | null>(null);
  const [creating, setCreating] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const load = useCallback(async () => {
    try {
      setPackages(await listPackages());
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
    if (!confirm("Delete this package?")) return;
    try {
      await deletePackage(id);
      setMessage({ type: "success", text: "Deleted" });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  async function handleClone(id: string) {
    try {
      const r = await clonePackage(id);
      setMessage({ type: "success", text: `Cloned as "${r.name}"` });
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
                <span className="text-[10px] font-mono text-zinc-500">{pkg.apt_packages?.length || 0} apt</span>
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
  pkg: Package | null;
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
      let targetId = pkg?.id;
      if (isEdit) {
        await updatePackage(pkg!.id, { name, description, db_kind: dbKind, db_version: dbVersion, apt_packages: aptPackages, pre_install: preInstall, custom_repo: customRepo, custom_repo_key: customRepoKey });
      } else {
        const res = await createPackage({ name, description, db_kind: dbKind, db_version: dbVersion, apt_packages: aptPackages, pre_install: preInstall, custom_repo: customRepo, custom_repo_key: customRepoKey });
        targetId = res.id;
      }
      // Upload .deb if selected (works for both create and edit).
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
