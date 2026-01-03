'use client';

import { useEffect, useState } from 'react';
import { Table, Column } from '@/components/ui/Table';
import { Button } from '@/components/ui/Button';
import { apiGet } from '@/lib/api';
import { formatDateTime, formatShortID } from '@/lib/format';
import { Plus, RefreshCw, Network } from 'lucide-react';

interface VPC {
  id: string;
  name: string;
  network_id: string;
  created_at: string;
}

export default function NetworkPage() {
  const [vpcs, setVpcs] = useState<VPC[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const loadVpcs = async () => {
    setLoading(true);
    setError('');
    try {
      const data = await apiGet<VPC[]>('/vpcs');
      setVpcs(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load VPCs');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadVpcs();
  }, []);

  const columns: Column<VPC>[] = [
    { 
      header: 'Name', 
      cell: (item) => (
         <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 500 }}>
          <Network size={16} color="var(--accent-orange)" />
          {item.name}
        </div>
      ) 
    },
    { header: 'VPC ID', cell: (item) => formatShortID(item.id) },
    { header: 'Network ID', cell: (item) => formatShortID(item.network_id) },
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
           <h1 style={{ fontSize: '34px', fontWeight: 700, marginBottom: '4px', letterSpacing: '0.01em', color: 'var(--text-primary)' }}>Network</h1>
           <p style={{ color: 'var(--text-secondary)' }}>Virtual Private Clouds and subnets.</p>
        </div>
        <div style={{ display: 'flex', gap: '12px' }}>
          <Button variant="secondary" onClick={loadVpcs} disabled={loading}><RefreshCw size={16} /></Button>
          <Button disabled><Plus size={16} style={{ marginRight: '8px' }} /> Create VPC</Button>
        </div>
      </header>

      {error ? (
        <div style={{ marginBottom: '16px', color: 'var(--accent-red)' }}>{error}</div>
      ) : null}
      <Table data={vpcs} columns={columns} />
    </div>
  );
}
