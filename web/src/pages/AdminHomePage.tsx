import { Link } from "react-router";

export function AdminHomePage() {
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Admin</h1>
      <ul className="space-y-2">
        <li><Link to="/admin/members" className="text-primary hover:underline">Manage members</Link></li>
        <li><Link to="/admin/frames" className="text-primary hover:underline">Manage frames</Link></li>
      </ul>
    </div>
  );
}
