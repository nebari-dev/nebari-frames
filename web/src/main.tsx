import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { Providers } from "@/app/Providers";
import { App } from "@/app/App";
import { applyBranding, loadBranding } from "@/lib/branding";
import "@fontsource-variable/geist";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "@/styles.css";

// Apply runtime branding (title, favicon, theme tokens) from the backend before
// the app mounts, so there is no flash of the default brand and the header reads
// the branded logo on its first render. Best-effort: an unreachable or invalid
// /branding.json leaves the app on its built-in Nebari defaults rather than
// blocking startup. Wrapped in a function rather than awaited at module scope
// because the build target (es2020) has no top-level await.
async function bootstrap() {
  try {
    applyBranding(await loadBranding());
  } catch {
    // Intentionally ignored - branding is never required to boot.
  }

  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <BrowserRouter>
        <Providers>
          <App />
        </Providers>
      </BrowserRouter>
    </StrictMode>,
  );
}

void bootstrap();
