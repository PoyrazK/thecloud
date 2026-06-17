import Link from 'next/link';

export default function AppNotFound() {
  return (
    <div style={{ padding: '24px 0', display: 'grid', gap: '10px' }}>
      <h2 style={{ margin: 0 }}>Page not found</h2>
      <p style={{ margin: 0, color: 'var(--muted-foreground)' }}>
        The requested console page does not exist.
      </p>
      <Link href="/dashboard" className="linkAccent">
        Go to dashboard
      </Link>
    </div>
  );
}
