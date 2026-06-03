import {
  createContext,
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  getAuthProvider,
  type RegisterInput,
  type SessionUser,
} from "@/services/auth";

export interface AuthContextValue {
  user: SessionUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (login: string, password: string) => Promise<SessionUser>;
  register: (input: RegisterInput) => Promise<SessionUser>;
  completeSSO: (
    providerId: string,
    code: string,
    state: string,
  ) => Promise<SessionUser>;
  logout: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextValue>({
  user: null,
  isAuthenticated: false,
  isLoading: true,
  login: async () => {
    throw new Error("AuthProvider not mounted");
  },
  register: async () => {
    throw new Error("AuthProvider not mounted");
  },
  completeSSO: async () => {
    throw new Error("AuthProvider not mounted");
  },
  logout: async () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<SessionUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const session = await getAuthProvider().getSession();
        if (!cancelled) setUser(session);
      } catch {
        if (!cancelled) setUser(null);
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const login = useCallback(async (login: string, password: string) => {
    const session = await getAuthProvider().login(login, password);
    setUser(session);
    return session;
  }, []);

  const register = useCallback(async (input: RegisterInput) => {
    const session = await getAuthProvider().register(input);
    setUser(session);
    return session;
  }, []);

  const completeSSO = useCallback(
    async (providerId: string, code: string, state: string) => {
      const session = await getAuthProvider().completeSSO(
        providerId,
        code,
        state,
      );
      setUser(session);
      return session;
    },
    [],
  );

  const logout = useCallback(async () => {
    try {
      await getAuthProvider().logout();
    } catch {
      /* best-effort */
    }
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        login,
        register,
        completeSSO,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
