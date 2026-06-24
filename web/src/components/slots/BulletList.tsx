export function BulletList({ items }: { items: string[] }) {
  return (
    <ul className="list-disc pl-5 space-y-1 text-sm">
      {items.map((item, i) => <li key={i}>{item}</li>)}
    </ul>
  );
}
