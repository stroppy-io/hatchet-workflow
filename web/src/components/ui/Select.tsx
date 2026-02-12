import { cn } from '../../App'

interface SelectProps extends Omit<React.SelectHTMLAttributes<HTMLSelectElement>, 'onChange' | 'value'> {
    label: string
    fieldName?: string
    value: any
    options: { label: string; value: any }[]
    onChange: (value: any) => void
}

export const Select = ({ label, fieldName, value, options, className, onChange, ...props }: SelectProps) => (
    <div className="space-y-1">
        <div className="flex justify-between items-baseline px-1">
            <label className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">{label}</label>
            {fieldName && <span className="text-[9px] font-mono text-primary/50">{fieldName}</span>}
        </div>
        <select
            {...props}
            value={value}
            onChange={(e) => onChange(typeof value === 'number' ? Number(e.target.value) : e.target.value)}
            className={cn(
                "w-full bg-background border border-input rounded-none px-3 py-2 text-xs font-mono text-foreground focus:border-primary outline-none transition-colors appearance-none",
                className
            )}
        >
            {options.map(opt => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
        </select>
    </div>
)
