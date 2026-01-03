'use client';

import { useEffect, useState } from 'react';
import { Table, Column } from '@/components/ui/Table';
import { Button } from '@/components/ui/Button';
import { apiGet } from '@/lib/api';
import { formatDateTime, formatShortID } from '@/lib/format';
import { Download, RefreshCw } from 'lucide-react';

interface Event {
  id: string;
  action: string;
  resource_id: string;
  resource_type: string;
  user_id: string;
  created_at: string;
}

export default function ActivityPage() {
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const loadEvents = async () => {
    setLoading(true);
    setError('');
    try {
      const data = await apiGet<Event[]>('/events');
      setEvents(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load events');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadEvents();
  }, []);

  const columns: Column<Event>[] = [
    { header: 'Action', accessorKey: 'action', width: '25%' },
    { header: 'Resource', cell: (item) => `${item.resource_type} ${formatShortID(item.resource_id)}`, width: '25%' },
    { header: 'User', cell: (item) => formatShortID(item.user_id) },
    { header: 'Timestamp', cell: (item) => formatDateTime(item.created_at) },
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
           <h1 style={{ fontSize: '34px', fontWeight: 700, marginBottom: '4px', letterSpacing: '0.01em', color: 'var(--text-primary)' }}>Activity</h1>
           <p style={{ color: 'var(--text-secondary)' }}>Audit logs and system events.</p>
        </div>
        <div style={{ display: 'flex', gap: '12px' }}>
          <Button variant="secondary" onClick={loadEvents} disabled={loading}><RefreshCw size={16} /></Button>
          <Button variant="secondary" disabled><Download size={16} style={{ marginRight: '8px' }} /> Export CSV</Button>
        </div>
      </header>

      {error ? (
        <div style={{ marginBottom: '16px', color: 'var(--accent-red)' }}>{error}</div>
      ) : null}
      <Table data={events} columns={columns} />
    </div>
  );
}
