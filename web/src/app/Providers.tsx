import { useMemo } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { TransportProvider } from "@connectrpc/connect-query";
import { AuthProvider } from "@/lib/auth/AuthProvider";
import { useAuth } from "@/lib/auth/useAuth";
import { ThemeProvider } from "@/lib/theme/ThemeContext";
import { createTransport } from "@/lib/transport";
import { queryClient } from "@/lib/query";

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
    <ThemeProvider>
      <AuthProvider>
        <TransportLayer>{children}</TransportLayer>
      </AuthProvider>
    </ThemeProvider>
  );
}
