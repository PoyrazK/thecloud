'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { Table, Column } from '@/components/ui/Table';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Plus, RefreshCw, Network } from 'lucide-react';
import { cloudApiRequest } from '@/lib/api';
import { useApiConfig } from '@/hooks/useApiConfig';
import styles from '../pages.module.css';

interface ApiVpc {
  id: string;
  name: string;
  cidr_block: string;
  status: string;
  network_id?: string;
  vxlan_id?: number;
  created_at: string;
}

function formatDate(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

export default function NetworkPage() {
  const { config, ready, hasCredentials } = useApiConfig();
  const [vpcs, setVpcs] = useState<ApiVpc[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [newVpcName, setNewVpcName] = useState('');
  const [newVpcCIDR, setNewVpcCIDR] = useState('10.0.0.0/16');
  const [pendingVpcId, setPendingVpcId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadVpcs = useCallback(async () => {
    if (!ready) return;

    if (!hasCredentials) {
      setVpcs([]);
      setIsLoading(false);
      setError(null);
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const response = await cloudApiRequest<ApiVpc[]>('/vpcs', undefined, config);
      setVpcs(response ?? []);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load VPCs.';
      setError(message);
    } finally {
      setIsLoading(false);
    }
  }, [config, hasCredentials, ready]);

  useEffect(() => {
    void loadVpcs();
  }, [loadVpcs]);

  const createVpc = async () => {
    if (!newVpcName.trim()) {
      setError('VPC name is required.');
      return;
    }

    setError(null);
    setIsSaving(true);
    try {
      await cloudApiRequest('/vpcs', {
        method: 'POST',
        body: JSON.stringify({
          name: newVpcName.trim(),
          cidr_block: newVpcCIDR.trim(),
        }),
      }, config);
      setNewVpcName('');
      await loadVpcs();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create VPC.';
      setError(message);
    } finally {
      setIsSaving(false);
    }
  };

  const deleteVpc = async (id: string) => {
    const allowed = window.confirm('Delete this VPC? This action cannot be undone.');
    if (!allowed) return;

    setError(null);
    setPendingVpcId(id);

    try {
      await cloudApiRequest(`/vpcs/${id}`, { method: 'DELETE' }, config);
      await loadVpcs();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete VPC.';
      setError(message);
    } finally {
      setPendingVpcId(null);
    }
  };

  const columns: Column<ApiVpc>[] = [
    { 
      header: 'Name', 
      cell: (item) => (
         <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Network size={16} color="var(--warn)" />
            <span>{item.name}</span>
          </div>
          <div className={styles.panelMeta}>{item.network_id || 'backend auto-assigned'}</div>
        </div>
      ) 
    },
    { header: 'VPC ID', accessorKey: 'id' },
    { header: 'IPv4 CIDR', accessorKey: 'cidr_block' },
    { 
      header: 'Status', 
      cell: (item) => {
        const normalized = item.status.toLowerCase();
        let indicator: 'running' | 'pending' | 'stopped' | 'error' = 'error';
        if (normalized.includes('active') || normalized.includes('running')) {
          indicator = 'running';
        } else if (normalized.includes('pending') || normalized.includes('creating') || normalized.includes('provisioning')) {
          indicator = 'pending';
        } else if (
          normalized.includes('stop') ||
          normalized.includes('deleted') ||
          normalized.includes('inactive') ||
          normalized.includes('terminated')
        ) {
          indicator = 'stopped';
        }
        return (
          <StatusIndicator
            status={indicator}
            label={item.status.toLowerCase()}
          />
        );
      },
    },
    {
      header: 'VXLAN',
      cell: (item) => item.vxlan_id ?? '-',
    },
    {
      header: 'Created',
      cell: (item) => formatDate(item.created_at),
    },
    {
      header: 'Actions',
      cell: (item) => (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => void deleteVpc(item.id)}
          loading={pendingVpcId === item.id}
          disabled={pendingVpcId === item.id}
        >
          Delete
        </Button>
      ),
    },
  ];

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Network</h1>
          <p className={styles.subtitle}>Provision tenant-isolated VPC segments and inspect networking metadata.</p>
        </div>
        <div className={styles.headerActions}>
          <Button variant="secondary" onClick={() => void loadVpcs()} loading={isLoading}>
            <RefreshCw size={16} /> Refresh
          </Button>
        </div>
      </header>

      {!hasCredentials ? (
        <div className={styles.notice}>
          <div>
            <strong>Network API access is not configured.</strong>
            <p className={styles.noticeText}>Add your API key and tenant details in Settings to load VPC resources.</p>
          </div>
          <Link href="/settings" className="linkAccent">
            Go to Settings
          </Link>
        </div>
      ) : null}

      {error ? <div className={styles.error}>{error}</div> : null}

      <Card title="Create VPC" subtitle="Provision a new isolated network" className={styles.panel}>
        <div className={styles.formRow}>
          <div className={styles.field}>
            <label htmlFor="vpcName">Name</label>
            <input
              id="vpcName"
              className={styles.input}
              placeholder="e.g. prod-network"
              value={newVpcName}
              onChange={(event) => setNewVpcName(event.target.value)}
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="vpcCidr">CIDR Block</label>
            <input
              id="vpcCidr"
              className={styles.input}
              placeholder="e.g. 10.20.0.0/16"
              value={newVpcCIDR}
              onChange={(event) => setNewVpcCIDR(event.target.value)}
            />
          </div>
          <div className={styles.field}>
            <span className={styles.panelMeta}>Action</span>
            <Button onClick={() => void createVpc()} loading={isSaving}>
              <Plus size={16} /> Create VPC
            </Button>
          </div>
        </div>
      </Card>

      <Card title="VPC Inventory" subtitle="Live results from /vpcs" className={styles.panel}>
        <Table
          data={vpcs}
          columns={columns}
          emptyMessage={isLoading ? 'Loading VPCs...' : 'No VPC records were returned by the API.'}
        />
      </Card>
    </div>
  );
}
