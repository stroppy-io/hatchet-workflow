import { cn } from '../../App'

interface InputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'onChange'> {
    label: string
    fieldName?: string
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => void
}

export const Input = ({ label, fieldName, className, onChange, ...props }: InputProps) => (
    <div className="space-y-1">
        <div className="flex justify-between items-baseline px-1">
            <label className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">{label}</label>
            {fieldName && <span className="text-[9px] font-mono text-primary/50">{fieldName}</span>}
        </div>
        <input
            {...props}
            onChange={onChange}
            className={cn(
                "w-full bg-background border border-input rounded-none px-3 py-2 text-xs font-mono text-foreground focus:border-primary outline-none transition-colors placeholder:opacity-20",
                className
            )}
        />
    </div>
)
