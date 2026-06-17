'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { Table, Column } from '@/components/ui/Table';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Download, RefreshCw } from 'lucide-react';
import { cloudApiRequest } from '@/lib/api';
import { eventStatus } from '@/lib/events';
import { useApiConfig } from '@/hooks/useApiConfig';
import styles from '../pages.module.css';

interface ApiEvent {
  id: string;
  action: string;
  resource_id: string;
  resource_type: string;
  metadata: unknown;
  created_at: string;
}

function formatDate(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function summarizeMetadata(metadata: unknown): string {
  if (!metadata) {
    return '-';
  }

  if (typeof metadata === 'string') {
    return metadata;
  }

  try {
    const summary = JSON.stringify(metadata);
    return summary.length > 110 ? `${summary.slice(0, 107)}...` : summary;
  } catch {
    return 'metadata unavailable';
  }
}

export default function ActivityPage() {
  const { config, ready, hasCredentials } = useApiConfig();
  const [events, setEvents] = useState<ApiEvent[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadEvents = useCallback(async () => {
    if (!ready) return;

    if (!hasCredentials) {
      setEvents([]);
      setIsLoading(false);
      setError(null);
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const response = await cloudApiRequest<ApiEvent[]>('/events?limit=120', undefined, config);
      setEvents(response ?? []);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load activity events.';
      setError(message);
    } finally {
      setIsLoading(false);
    }
  }, [config, hasCredentials, ready]);

  useEffect(() => {
    void loadEvents();
  }, [loadEvents]);

  const exportCsv = () => {
    if (events.length === 0) {
      return;
    }

    const escapeCsv = (value: unknown) => {
      const raw = String(value ?? '');
      const guarded = /^[=+\-@\t\r]/.test(raw) ? `'${raw}` : raw;
      return `"${guarded.replace(/"/g, '""')}"`;
    };

    const header = ['id', 'action', 'resource_type', 'resource_id', 'metadata', 'created_at'];
    const rows = events.map((event) => [
      event.id,
      event.action,
      event.resource_type,
      event.resource_id,
      summarizeMetadata(event.metadata),
      event.created_at,
    ]);

    const csv = [header, ...rows]
      .map((line) => line.map(escapeCsv).join(','))
      .join('\n');

    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `thecloud-events-${new Date().toISOString().slice(0, 10)}.csv`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    setTimeout(() => URL.revokeObjectURL(url), 0);
  };

  const columns: Column<ApiEvent>[] = [
    { header: 'Action', accessorKey: 'action', width: '20%' },
    {
      header: 'Resource',
      width: '24%',
      cell: (item) => (
        <div>
          <div>{item.resource_type}</div>
          <div className={styles.panelMeta}>{item.resource_id}</div>
        </div>
      ),
    },
    { 
      header: 'Status', 
      cell: (item) => (
        <StatusIndicator status={eventStatus(item.action)} label={eventStatus(item.action)} />
      )
    },
    {
      header: 'Metadata',
      width: '28%',
      cell: (item) => <span className={styles.panelMeta}>{summarizeMetadata(item.metadata)}</span>,
    },
    {
      header: 'Timestamp',
      width: '16%',
      cell: (item) => formatDate(item.created_at),
    },
  ];

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Activity</h1>
          <p className={styles.subtitle}>Live event timeline from backend audit and orchestration services.</p>
        </div>
        <div className={styles.headerActions}>
          <Button variant="secondary" onClick={() => void loadEvents()} loading={isLoading}>
            <RefreshCw size={16} /> Refresh
          </Button>
          <Button variant="secondary" onClick={exportCsv}>
            <Download size={16} /> Export CSV
          </Button>
        </div>
      </header>

      {!hasCredentials ? (
        <div className={styles.notice}>
          <div>
            <strong>Activity API access is not configured.</strong>
            <p className={styles.noticeText}>Add API key and tenant details in Settings to load event streams.</p>
          </div>
          <Link href="/settings" className="linkAccent">
            Go to Settings
          </Link>
        </div>
      ) : null}

      {error ? <div className={styles.error}>{error}</div> : null}

      <Card title="Event Stream" subtitle="Live results from /events" className={styles.panel}>
        <Table
          data={events}
          columns={columns}
          emptyMessage={isLoading ? 'Loading activity...' : 'No events were returned by the API.'}
        />
      </Card>
    </div>
  );
}
