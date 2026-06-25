import { Navigate, Outlet } from "react-router";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";

export function RequireAdmin() {
  const { isLoading, data } = useQuery(FrameService.method.getMe, {});
  if (isLoading) {
    return <div className="p-8 text-muted-foreground">Loading...</div>;
  }
  if (data?.role !== "admin") {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}
