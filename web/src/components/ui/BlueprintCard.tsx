import { motion } from 'framer-motion'
import { cn } from '../../App'
import type { LucideIcon } from 'lucide-react'

interface BlueprintCardProps {
    title: string
    fieldName?: string
    subtitle: string
    icon: LucideIcon
    active: boolean
    onClick: () => void
    children?: React.ReactNode
}

export const BlueprintCard = ({ title, fieldName, subtitle, icon: Icon, active, onClick, children }: BlueprintCardProps) => (
    <div className={cn(
        "border-2 transition-all p-6 relative overflow-hidden",
        active ? "bg-primary/5 border-primary shadow-[0_0_30px_rgba(var(--primary),0.1)]" : "bg-card/20 border-border opacity-60 grayscale hover:opacity-100 hover:grayscale-0"
    )}>
        <div className="flex justify-between items-start mb-6">
            <div className="flex items-center gap-3">
                <div className={cn("p-2 rounded-none skew-x-[-12deg]", active ? "bg-primary text-white" : "bg-muted text-muted-foreground")}>
                    <Icon className="w-5 h-5 skew-x-[12deg]" />
                </div>
                <div>
                    <h4 className="text-xs font-black uppercase italic tracking-tighter">{title}</h4>
                    {fieldName && <p className="text-[9px] font-mono text-primary/70">{fieldName}</p>}
                    <p className="text-[8px] text-muted-foreground uppercase font-bold mt-1">{subtitle}</p>
                </div>
            </div>
            <input type="checkbox" checked={active} onChange={onClick} className="w-4 h-4 accent-primary cursor-pointer" />
        </div>
        {active && <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-4">{children}</motion.div>}
    </div>
)
