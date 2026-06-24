import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor, Transport } from "@connectrpc/connect";
import { env } from "@/lib/env";

// authHeaderInterceptor attaches the bearer token (if any) to every request.
export function authHeaderInterceptor(getToken: () => string | undefined): Interceptor {
  return (next) => async (req) => {
    const token = getToken();
    if (token) {
      req.header.set("Authorization", `Bearer ${token}`);
    }
    return next(req);
  };
}

export function createTransport(getToken: () => string | undefined): Transport {
  return createConnectTransport({
    baseUrl: env.backendBaseUrl,
    interceptors: [authHeaderInterceptor(getToken)],
  });
}
