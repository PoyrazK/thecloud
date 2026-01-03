'use client';

import { useEffect, useState } from 'react';
import { Table, Column } from '@/components/ui/Table';
import { Button } from '@/components/ui/Button';
import { apiGet } from '@/lib/api';
import { formatDateTime } from '@/lib/format';
import { Plus, RefreshCw, HardDrive } from 'lucide-react';

interface StorageObject {
  id: string;
  bucket: string;
  key: string;
  size_bytes: number;
  content_type: string;
  created_at: string;
}

export default function StoragePage() {
  const [bucket, setBucket] = useState('test-bucket');
  const [objects, setObjects] = useState<StorageObject[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const loadObjects = async () => {
    if (!bucket.trim()) {
      setError('Bucket name is required');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const data = await apiGet<StorageObject[]>(`/storage/${bucket.trim()}`);
      setObjects(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load objects');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadObjects();
  }, []);

  const columns: Column<StorageObject>[] = [
    { 
      header: 'Object Key', 
      cell: (item) => (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 500 }}>
          <HardDrive size={16} color="var(--accent-blue)" />
          {item.key}
        </div>
      )
    },
    { header: 'Size (bytes)', cell: (item) => item.size_bytes.toLocaleString() },
    { header: 'Content Type', accessorKey: 'content_type' },
    { header: 'Created', cell: (item) => formatDateTime(item.created_at) },
  ];

  return (
    <div style={{ maxWidth: '1280px', margin: '0 auto' }}>
      <header style={{ 
        display: 'flex', 
        justifyContent: 'space-between', 
        alignItems: 'center',
        marginBottom: '32px' 
      }}>
        <div>
           <h1 style={{ fontSize: '34px', fontWeight: 700, marginBottom: '4px', letterSpacing: '0.01em', color: 'var(--text-primary)' }}>Storage</h1>
           <p style={{ color: 'var(--text-secondary)' }}>S3-compatible object storage.</p>
        </div>
        <div style={{ display: 'flex', gap: '12px' }}>
          <Button variant="secondary" onClick={loadObjects} disabled={loading}><RefreshCw size={16} /></Button>
          <Button disabled><Plus size={16} style={{ marginRight: '8px' }} /> Create Bucket</Button>
        </div>
      </header>

      <div style={{ display: 'flex', gap: '12px', marginBottom: '16px' }}>
        <input
          value={bucket}
          onChange={(event) => setBucket(event.target.value)}
          placeholder="Bucket name"
          style={{
            flex: 1,
            padding: '10px 12px',
            borderRadius: '8px',
            border: '1px solid var(--glass-border)',
            background: 'var(--system-gray-6)',
            color: 'var(--text-primary)',
            fontSize: '14px'
          }}
        />
        <Button variant="secondary" onClick={loadObjects} disabled={loading}>Load Objects</Button>
      </div>

      {error ? (
        <div style={{ marginBottom: '16px', color: 'var(--accent-red)' }}>{error}</div>
      ) : null}
      <Table data={objects} columns={columns} />
    </div>
  );
}
