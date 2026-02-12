import { create } from '@bufbuild/protobuf'
import { HardwareSchema } from '../../proto/deployment/deployment_pb'

interface HardwareInputsProps {
    label: string
    fieldName?: string
    hardware: any
    onChange: (h: any) => void
}

export const HardwareInputs = ({ label, fieldName, hardware, onChange }: HardwareInputsProps) => (
    <div className="space-y-2">
        <div className="flex justify-between items-baseline px-1">
            <span className="text-[9px] font-black uppercase tracking-widest text-primary/70">{label}</span>
            {fieldName && <span className="text-[9px] font-mono text-primary/50">{fieldName}</span>}
        </div>
        <div className="grid grid-cols-3 gap-2">
            <div className="bg-background border border-input p-2 flex flex-col items-center">
                <span className="text-[7px] font-bold text-muted-foreground uppercase">Cores</span>
                <input
                    type="number"
                    value={hardware?.cores || 0}
                    onChange={(e) => onChange(create(HardwareSchema, { ...hardware, cores: parseInt(e.target.value) }))}
                    className="w-full text-center bg-transparent text-xs font-black outline-none"
                />
            </div>
            <div className="bg-background border border-input p-2 flex flex-col items-center">
                <span className="text-[7px] font-bold text-muted-foreground uppercase">RAM</span>
                <input
                    type="number"
                    value={hardware?.memory || 0}
                    onChange={(e) => onChange(create(HardwareSchema, { ...hardware, memory: parseInt(e.target.value) }))}
                    className="w-full text-center bg-transparent text-xs font-black outline-none"
                />
            </div>
            <div className="bg-background border border-input p-2 flex flex-col items-center">
                <span className="text-[7px] font-bold text-muted-foreground uppercase">Disk</span>
                <input
                    type="number"
                    value={hardware?.disk || 0}
                    onChange={(e) => onChange(create(HardwareSchema, { ...hardware, disk: parseInt(e.target.value) }))}
                    className="w-full text-center bg-transparent text-xs font-black outline-none"
                />
            </div>
        </div>
    </div>
)
