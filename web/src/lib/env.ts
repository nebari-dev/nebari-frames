// Same-origin in production (SPA embedded in the Go binary) and in dev
// (Vite proxy forwards RPC + /auth/config to the backend). An empty base
// URL means "current origin".
export const env = {
  backendBaseUrl: "",
};
