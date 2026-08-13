import { useMemo } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { TransportProvider } from "@connectrpc/connect-query";
import { ThemeProvider } from "@/hooks/theme-provider";
import { AuthProvider } from "@/lib/auth/AuthProvider";
import { useAuth } from "@/lib/auth/useAuth";
import { createTransport } from "@/lib/transport";
import { queryClient } from "@/lib/query";

/**
 * localStorage key the theme preference persists under. Predates the registry
 * hook (whose default is "nebari:themeMode") — keep it so users don't lose
 * their saved preference. The inline bootstrap script in index.html reads the
 * same key; update both together.
 */
export const THEME_STORAGE_KEY = "nebari-frames:themeMode";

// Inner component: builds the transport bound to the current auth context.
function TransportLayer({ children }: { children: React.ReactNode }) {
  const auth = useAuth();
  const transport = useMemo(
    () =>
      createTransport(
        () => auth.getAccessToken(),
        // oidc-client-ts handles silent renew; re-read the current token.
        async () => auth.getAccessToken(),
      ),
    [auth],
  );
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </TransportProvider>
  );
}

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider storageKey={THEME_STORAGE_KEY}>
      <AuthProvider>
        <TransportLayer>{children}</TransportLayer>
      </AuthProvider>
    </ThemeProvider>
  );
}
