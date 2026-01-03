'use client';

import { useEffect, useState } from 'react';
import { Table, Column } from '@/components/ui/Table';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Button } from '@/components/ui/Button';
import { apiGet } from '@/lib/api';
import { formatDateTime, formatShortID } from '@/lib/format';
import { RefreshCw } from 'lucide-react';

interface Instance {
  id: string;
  name: string;
  image: string;
  status: string;
  ports?: string;
  created_at: string;
}

type IndicatorStatus = 'running' | 'stopped' | 'pending' | 'error';

const statusToIndicator = (status: string): IndicatorStatus => {
  switch (status.toUpperCase()) {
    case 'RUNNING':
      return 'running';
    case 'STOPPED':
      return 'stopped';
    case 'ERROR':
      return 'error';
    case 'STARTING':
    default:
      return 'pending';
  }
};

export default function ComputePage() {
  const [instances, setInstances] = useState<Instance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const loadInstances = async () => {
    setLoading(true);
    setError('');
    try {
      const data = await apiGet<Instance[]>('/instances');
      setInstances(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load instances');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadInstances();
  }, []);

  const columns: Column<Instance>[] = [
    { header: 'Name', accessorKey: 'name', width: '25%' },
    {
      header: 'Instance ID',
      cell: (item) => formatShortID(item.id),
      width: '15%',
    },
    { header: 'Image', accessorKey: 'image', width: '20%' },
    { 
      header: 'Status', 
      cell: (item) => <StatusIndicator status={statusToIndicator(item.status)} label={item.status.toLowerCase()} /> 
    },
    { header: 'Ports', cell: (item) => item.ports || '-' },
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
           <h1 style={{ fontSize: '34px', fontWeight: 700, marginBottom: '4px', letterSpacing: '0.01em', color: 'var(--text-primary)' }}>Compute</h1>
           <p style={{ color: 'var(--text-secondary)' }}>Manage your virtual machines.</p>
        </div>
        <div style={{ display: 'flex', gap: '12px' }}>
          <Button variant="secondary" onClick={loadInstances} disabled={loading}><RefreshCw size={16} /></Button>
        </div>
      </header>

      {error ? (
        <div style={{ marginBottom: '16px', color: 'var(--accent-red)' }}>{error}</div>
      ) : null}
      <Table data={instances} columns={columns} />
    </div>
  );
}
