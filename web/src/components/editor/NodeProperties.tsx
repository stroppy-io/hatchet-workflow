import React from "react"
import { useStore } from "@nanostores/react"
import { motion } from "framer-motion"
import { Settings, Trash2 } from "lucide-react"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  $currentTest,
  $selectedNodeId,
  updateNode,
  removeNode,
  type PostgresNode,
  type Hardware,
} from "@/stores/editor"

function FieldGroup({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <label className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
        {label}
      </label>
      {children}
    </div>
  )
}

function Toggle({
  label,
  checked,
  onChange,
}: {
  label: string
  checked: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <div className="flex items-center gap-2">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="h-3.5 w-3.5 accent-primary"
      />
      <span className="text-[12px]">{label}</span>
    </div>
  )
}

export function NodeProperties() {
  const test = useStore($currentTest)
  const selectedNodeId = useStore($selectedNodeId)

  if (!selectedNodeId || !test?.databaseTemplate) return null

  const node = test.databaseTemplate.nodes.find((n) => n.name === selectedNodeId)
  if (!node) return null

  function patch(p: Partial<PostgresNode>) {
    updateNode(selectedNodeId!, p)
  }

  function patchHw(p: Partial<Hardware>) {
    patch({ hardware: { ...node!.hardware, ...p } })
  }

  function handleDelete() {
    removeNode(selectedNodeId!)
  }

  return (
    <motion.div
      initial={{ opacity: 0, x: 16 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ duration: 0.15 }}
      className="flex flex-col h-full border-l border-border bg-card"
    >
      <div className="flex items-center gap-2 border-b border-border bg-secondary/20 px-3 py-1.5">
        <Settings className="h-3.5 w-3.5 text-primary" />
        <span className="text-[12px] font-medium">Node Properties</span>
      </div>

      <div className="flex flex-col gap-3 p-3 overflow-y-auto flex-1">
        <FieldGroup label="Name">
          <Input
            value={node.name}
            onChange={(e) => patch({ name: e.target.value })}
            className="bg-background h-7 text-[12px]"
          />
        </FieldGroup>

        <FieldGroup label="Role">
          <Badge
            variant={node.role === "MASTER" ? "default" : "secondary"}
            className="text-[10px]"
          >
            {node.role}
          </Badge>
        </FieldGroup>

        <div className="border-t border-border pt-2">
          <span className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
            Hardware
          </span>
          <div className="grid grid-cols-1 gap-2 mt-1.5">
            <FieldGroup label="Cores">
              <Input
                type="number"
                value={node.hardware.cores}
                onChange={(e) => patchHw({ cores: parseInt(e.target.value) || 0 })}
                className="bg-background h-7 text-[12px]"
              />
            </FieldGroup>
            <FieldGroup label="Memory (GB)">
              <Input
                type="number"
                value={node.hardware.memory}
                onChange={(e) => patchHw({ memory: parseInt(e.target.value) || 0 })}
                className="bg-background h-7 text-[12px]"
              />
            </FieldGroup>
            <FieldGroup label="Disk (GB)">
              <Input
                type="number"
                value={node.hardware.disk}
                onChange={(e) => patchHw({ disk: parseInt(e.target.value) || 0 })}
                className="bg-background h-7 text-[12px]"
              />
            </FieldGroup>
          </div>
        </div>

        <div className="border-t border-border pt-2 space-y-1.5">
          <span className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
            Services
          </span>
          <Toggle
            label="PgBouncer"
            checked={node.hasPgbouncer}
            onChange={(v) => patch({ hasPgbouncer: v })}
          />
          <Toggle
            label="Etcd"
            checked={node.hasEtcd}
            onChange={(v) => patch({ hasEtcd: v })}
          />
          <Toggle
            label="Monitoring"
            checked={node.hasMonitoring}
            onChange={(v) => patch({ hasMonitoring: v })}
          />
          <Toggle
            label="Backup"
            checked={node.hasBackup}
            onChange={(v) => patch({ hasBackup: v })}
          />
        </div>
      </div>

      <div className="p-3 border-t border-border">
        <Button
          variant="destructive"
          size="sm"
          className="w-full text-[12px]"
          onClick={handleDelete}
        >
          <Trash2 className="h-3 w-3" />
          Delete Node
        </Button>
      </div>
    </motion.div>
  )
}
