import { Navigate, Outlet } from "react-router";
import { useQuery } from "@connectrpc/connect-query";
import { Code, ConnectError } from "@connectrpc/connect";
import { FrameService } from "@gen/frames/v1/frame_service_pb";

export function RequireMembership() {
  const { isLoading, error } = useQuery(FrameService.method.getMe, {});
  if (isLoading) {
    return <div className="p-8 text-muted-foreground">Loading...</div>;
  }
  if (error) {
    const code = ConnectError.from(error).code;
    if (code === Code.PermissionDenied) {
      return <Navigate to="/no-access" replace />;
    }
    if (code === Code.Unauthenticated) {
      return <Navigate to="/login" replace />;
    }
    return <div className="p-8 text-destructive">Something went wrong.</div>;
  }
  return <Outlet />;
}
