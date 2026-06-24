export function NoAccessPage() {
  return (
    <div className="min-h-screen grid place-items-center px-4">
      <div className="max-w-md text-center space-y-3">
        <h1 className="text-xl font-semibold">No organization access</h1>
        <p className="text-muted-foreground">
          Your account is not a member of any organization yet. Ask an
          organization admin to add you, then reload this page.
        </p>
      </div>
    </div>
  );
}
