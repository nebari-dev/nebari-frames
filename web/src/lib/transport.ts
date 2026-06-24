import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor, Transport } from "@connectrpc/connect";
import { Code, ConnectError } from "@connectrpc/connect";
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

// On an unauthenticated response, refresh the token once and retry.
export function refreshOn401Interceptor(
  refresh: () => Promise<string | undefined>,
): Interceptor {
  return (next) => async (req) => {
    try {
      return await next(req);
    } catch (err) {
      if (ConnectError.from(err).code !== Code.Unauthenticated) {
        throw err;
      }
      const token = await refresh();
      if (token) {
        req.header.set("Authorization", `Bearer ${token}`);
      }
      return next(req);
    }
  };
}

export function createTransport(
  getToken: () => string | undefined,
  onUnauthorized?: () => Promise<string | undefined>,
): Transport {
  const interceptors: Interceptor[] = [authHeaderInterceptor(getToken)];
  if (onUnauthorized) {
    interceptors.push(refreshOn401Interceptor(onUnauthorized));
  }
  return createConnectTransport({ baseUrl: env.backendBaseUrl, interceptors });
}
