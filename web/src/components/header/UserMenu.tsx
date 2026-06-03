import {
  LogOut,
  Settings as SettingsIcon,
  Building2,
  SlidersHorizontal,
  UsersRound,
  ShieldCheck,
  ChevronDown,
} from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { Avatar } from "@/components/Avatar";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";

// Account menu (right, avatar dropdown). Holds the user identity, the
// Organizations entry, profile/settings, and logout. For platform admins it also
// hosts the admin API group.
const platformItems = [
  { to: "/admin/accounts", icon: UsersRound, label: "Accounts" },
  {
    to: "/admin/identity-providers",
    icon: ShieldCheck,
    label: "Identity providers",
  },
  { to: "/admin/system", icon: SlidersHorizontal, label: "System settings" },
] as const;

export function UserMenu() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  if (!user) return null;
  const display = user.displayName || user.username;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="flex max-w-[12rem] items-center gap-2 rounded-full pr-1 outline-none focus-visible:ring-1 focus-visible:ring-ring">
        <Avatar name={user.username} size={28} />
        <span className="truncate text-sm font-medium text-foreground">
          {display}
        </span>
        <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-[15rem]">
        <DropdownMenuLabel>Signed in as</DropdownMenuLabel>
        <div className="flex items-center gap-2.5 px-3 pb-2 pt-0.5">
          <Avatar name={user.username} size={32} />
          <div className="flex min-w-0 flex-col">
            <span className="truncate text-sm text-foreground">{display}</span>
            {user.email && (
              <span className="truncate text-[10px] text-muted-foreground">
                {user.email}
              </span>
            )}
          </div>
        </div>

        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => navigate("/orgs")}>
          <Building2 className="h-4 w-4" />
          <span>Organizations</span>
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => navigate("/profile")}>
          <SettingsIcon className="h-4 w-4" />
          <span>Account settings</span>
        </DropdownMenuItem>

        {user.isAdmin && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuLabel>Admin</DropdownMenuLabel>
            {platformItems.map((item) => (
              <DropdownMenuItem
                key={item.to}
                onSelect={() => navigate(item.to)}
              >
                <item.icon className="h-4 w-4" />
                <span>{item.label}</span>
              </DropdownMenuItem>
            ))}
          </>
        )}

        <DropdownMenuSeparator />
        <DropdownMenuItem
          onSelect={() => {
            void logout();
          }}
          className="text-destructive focus:text-destructive"
        >
          <LogOut className="h-4 w-4" />
          <span>Sign out</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
