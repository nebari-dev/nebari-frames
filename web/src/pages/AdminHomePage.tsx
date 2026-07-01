import { Link } from "react-router";

export function AdminHomePage() {
  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">Admin</h1>
        <p className="text-muted-foreground">
          Manage members and frames for your Nebari Frames instance.
        </p>
      </div>
      <ul className="space-y-2">
        <li><Link to="/admin/members" className="text-primary hover:underline">Manage members</Link></li>
        <li><Link to="/admin/frames" className="text-primary hover:underline">Manage frames</Link></li>
        <li><Link to="/hierarchy" className="text-primary hover:underline">Frame hierarchy</Link></li>
      </ul>
    </div>
  );
}
