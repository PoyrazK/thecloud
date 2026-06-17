'use client';

import { useEffect } from 'react';
import Link from 'next/link';

export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <div style={{ padding: '24px 0', display: 'grid', gap: '12px' }}>
      <h2 style={{ margin: 0 }}>Something went wrong in the console.</h2>
      <p style={{ margin: 0, color: 'var(--muted-foreground)' }}>
        Something went wrong. Please try again.
      </p>
      {error.digest ? <p style={{ margin: 0, color: 'var(--muted-foreground)' }}>Reference: {error.digest}</p> : null}
      <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
        <button
          type="button"
          onClick={reset}
          style={{
            height: '36px',
            padding: '0 12px',
            borderRadius: '10px',
            border: '1px solid rgba(15,23,42,0.15)',
            background: '#fff',
            cursor: 'pointer',
          }}
        >
          Try again
        </button>
        <Link href="/dashboard" className="linkAccent">
          Back to dashboard
        </Link>
      </div>
    </div>
  );
}
