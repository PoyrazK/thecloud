'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { Table, Column } from '@/components/ui/Table';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Button } from '@/components/ui/Button';
import { LaunchInstanceModal } from '@/components/compute/LaunchInstanceModal';
import { Card } from '@/components/ui/Card';
import { Plus, RefreshCw } from 'lucide-react';
import { cloudApiRequest } from '@/lib/api';
import { useApiConfig } from '@/hooks/useApiConfig';
import styles from '../pages.module.css';

interface ApiInstance {
  id: string;
  name: string;
  image?: string;
  instance_type?: string;
  status: string;
  private_ip?: string;
  created_at: string;
}

interface ApiVpc {
  id: string;
  name: string;
  cidr_block?: string;
}

interface LaunchPayload {
  name: string;
  image: string;
  ports: string;
  vpcId?: string;
}

type InstanceIndicator = 'running' | 'stopped' | 'pending' | 'error';

function mapStatus(status: string): InstanceIndicator {
  const normalized = status.toLowerCase();
  if (normalized.includes('running')) return 'running';
  if (normalized.includes('stop') || normalized.includes('deleted')) return 'stopped';
  if (normalized.includes('start') || normalized.includes('pending') || normalized.includes('creating')) {
    return 'pending';
  }
  return 'error';
}

function formatDate(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

export default function ComputePage() {
  const { config, ready, hasCredentials } = useApiConfig();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [instances, setInstances] = useState<ApiInstance[]>([]);
  const [vpcs, setVpcs] = useState<ApiVpc[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isLaunching, setIsLaunching] = useState(false);
  const [pendingInstanceIDs, setPendingInstanceIDs] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    if (!ready) return;
    if (!hasCredentials) {
      setInstances([]);
      setVpcs([]);
      setIsLoading(false);
      setError(null);
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const [instanceData, vpcData] = await Promise.all([
        cloudApiRequest<ApiInstance[]>('/instances', undefined, config),
        cloudApiRequest<ApiVpc[]>('/vpcs', undefined, config),
      ]);
      setInstances(instanceData ?? []);
      setVpcs(vpcData ?? []);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load instances.';
      setError(message);
    } finally {
      setIsLoading(false);
    }
  }, [config, hasCredentials, ready]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const runningCount = useMemo(
    () => instances.filter((instance) => mapStatus(instance.status) === 'running').length,
    [instances]
  );

  const handleLaunch = async (data: LaunchPayload) => {
    setIsLaunching(true);
    setError(null);

    const payload: Record<string, string> = {
      name: data.name,
      image: data.image,
    };

    if (data.ports) {
      payload.ports = data.ports;
    }

    if (data.vpcId) {
      payload.vpc_id = data.vpcId;
    }

    try {
      await cloudApiRequest('/instances', {
        method: 'POST',
        body: JSON.stringify(payload),
      }, config);
      await loadData();
      setIsModalOpen(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to launch instance.';
      setError(message);
      throw err;
    } finally {
      setIsLaunching(false);
    }
  };

  const stopInstance = async (id: string) => {
    setPendingInstanceIDs((previous) => {
      const next = new Set(previous);
      next.add(id);
      return next;
    });
    setError(null);
    try {
      await cloudApiRequest(`/instances/${id}/stop`, { method: 'POST' }, config);
      await loadData();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to stop instance.';
      setError(message);
    } finally {
      setPendingInstanceIDs((previous) => {
        const next = new Set(previous);
        next.delete(id);
        return next;
      });
    }
  };

  const terminateInstance = async (id: string) => {
    setPendingInstanceIDs((previous) => {
      const next = new Set(previous);
      next.add(id);
      return next;
    });
    setError(null);
    try {
      await cloudApiRequest(`/instances/${id}`, { method: 'DELETE' }, config);
      await loadData();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to terminate instance.';
      setError(message);
    } finally {
      setPendingInstanceIDs((previous) => {
        const next = new Set(previous);
        next.delete(id);
        return next;
      });
    }
  };

  const columns: Column<ApiInstance>[] = [
    {
      header: 'Name',
      width: '22%',
      cell: (item) => (
        <div>
          <div>{item.name}</div>
          <div className={styles.panelMeta}>{item.image ?? 'custom image'}</div>
        </div>
      ),
    },
    { header: 'Instance ID', accessorKey: 'id', width: '22%' },
    {
      header: 'Type',
      width: '13%',
      cell: (item) => item.instance_type ?? 'standard',
    },
    { 
      header: 'Status', 
      width: '13%',
      cell: (item) => <StatusIndicator status={mapStatus(item.status)} label={item.status.toLowerCase()} /> 
    },
    {
      header: 'Private IP',
      width: '11%',
      cell: (item) => item.private_ip || '-',
    },
    {
      header: 'Created',
      width: '13%',
      cell: (item) => formatDate(item.created_at),
    },
    {
      header: 'Actions',
      width: '16%',
      cell: (item) => (
        <div className={styles.headerActions}>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void stopInstance(item.id)}
            loading={pendingInstanceIDs.has(item.id)}
            disabled={pendingInstanceIDs.has(item.id)}
          >
            Stop
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void terminateInstance(item.id)}
            loading={pendingInstanceIDs.has(item.id)}
            disabled={pendingInstanceIDs.has(item.id)}
          >
            Delete
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Compute</h1>
          <p className={styles.subtitle}>Manage real instances with live backend synchronization.</p>
        </div>
        <div className={styles.headerActions}>
          <Button variant="secondary" onClick={() => void loadData()} loading={isLoading}>
            <RefreshCw size={16} />
            Refresh
          </Button>
          <Button onClick={() => setIsModalOpen(true)}>
            <Plus size={16} /> Launch Instance
          </Button>
        </div>
      </header>

      {!hasCredentials ? (
        <div className={styles.notice}>
          <div>
            <strong>Compute API access is not configured.</strong>
            <p className={styles.noticeText}>Add API key and tenant details in Settings to query live instances.</p>
          </div>
          <Link href="/settings" className="linkAccent">
            Go to Settings
          </Link>
        </div>
      ) : null}

      {error ? <div className={styles.error}>{error}</div> : null}

      <section className={styles.statsGrid}>
        <article className={styles.stat}>
          <div className={styles.statLabel}>Total Instances</div>
          <div className={styles.statValue}>{instances.length}</div>
          <div className={styles.statHint}>All known resources</div>
        </article>
        <article className={styles.stat}>
          <div className={styles.statLabel}>Running</div>
          <div className={styles.statValue}>{runningCount}</div>
          <div className={styles.statHint}>Healthy active compute</div>
        </article>
        <article className={styles.stat}>
          <div className={styles.statLabel}>Stopped / Other</div>
          <div className={styles.statValue}>{Math.max(instances.length - runningCount, 0)}</div>
          <div className={styles.statHint}>Needs intervention or idle</div>
        </article>
        <article className={styles.stat}>
          <div className={styles.statLabel}>VPC Options</div>
          <div className={styles.statValue}>{vpcs.length}</div>
          <div className={styles.statHint}>Available attach targets</div>
        </article>
      </section>

      <Card
        title="Instance Inventory"
        subtitle="Live results from /instances"
        className={styles.panel}
      >
        <Table
          data={instances}
          columns={columns}
          emptyMessage={isLoading ? 'Loading instances...' : 'No instances were returned by the API.'}
        />
      </Card>

      <LaunchInstanceModal 
        isOpen={isModalOpen} 
        onClose={() => setIsModalOpen(false)}
        onSubmit={handleLaunch}
        isSubmitting={isLaunching}
        vpcs={vpcs}
      />
    </div>
  );
}
