'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { Table, Column } from '@/components/ui/Table';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Plus, RefreshCw, HardDrive } from 'lucide-react';
import { cloudApiRequest } from '@/lib/api';
import { useApiConfig } from '@/hooks/useApiConfig';
import styles from '../pages.module.css';

interface ApiBucket {
  id: string;
  name: string;
  is_public: boolean;
  versioning_enabled: boolean;
  encryption_enabled: boolean;
  created_at: string;
}

function formatDate(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

export default function StoragePage() {
  const { config, ready, hasCredentials } = useApiConfig();
  const [buckets, setBuckets] = useState<ApiBucket[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [newBucketName, setNewBucketName] = useState('');
  const [createPublic, setCreatePublic] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadBuckets = useCallback(async () => {
    if (!ready) return;

    if (!hasCredentials) {
      setBuckets([]);
      setIsLoading(false);
      setError(null);
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const bucketData = await cloudApiRequest<ApiBucket[]>('/storage/buckets', undefined, config);
      setBuckets(bucketData ?? []);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load buckets.';
      setError(message);
    } finally {
      setIsLoading(false);
    }
  }, [config, hasCredentials, ready]);

  useEffect(() => {
    void loadBuckets();
  }, [loadBuckets]);

  const createBucket = async () => {
    if (!newBucketName.trim()) {
      setError('Bucket name is required.');
      return;
    }

    setError(null);
    setIsSaving(true);
    try {
      await cloudApiRequest('/storage/buckets', {
        method: 'POST',
        body: JSON.stringify({
          name: newBucketName.trim(),
          is_public: createPublic,
        }),
      }, config);
      setNewBucketName('');
      setCreatePublic(false);
      await loadBuckets();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create bucket.';
      setError(message);
    } finally {
      setIsSaving(false);
    }
  };

  const deleteBucket = async (name: string) => {
    const allowed = window.confirm(`Delete bucket "${name}"?`);
    if (!allowed) return;

    setError(null);

    try {
      await cloudApiRequest(`/storage/buckets/${encodeURIComponent(name)}`, {
        method: 'DELETE',
      }, config);
      await loadBuckets();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete bucket.';
      setError(message);
    }
  };

  const columns: Column<ApiBucket>[] = [
    { 
      header: 'Name', 
      cell: (item) => (
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <HardDrive size={15} color="var(--brand)" />
            <span>{item.name}</span>
          </div>
          <div className={styles.panelMeta}>{item.id}</div>
        </div>
      )
    },
    {
      header: 'Visibility',
      cell: (item) => (
        <span className={`${styles.badge} ${item.is_public ? styles.badgeWarn : styles.badgeNeutral}`}>
          {item.is_public ? 'Public' : 'Private'}
        </span>
      ),
    },
    {
      header: 'Versioning',
      cell: (item) => (
        <span className={`${styles.badge} ${item.versioning_enabled ? styles.badgeGood : styles.badgeNeutral}`}>
          {item.versioning_enabled ? 'Enabled' : 'Disabled'}
        </span>
      ),
    },
    {
      header: 'Encryption',
      cell: (item) => (
        <span className={`${styles.badge} ${item.encryption_enabled ? styles.badgeGood : styles.badgeNeutral}`}>
          {item.encryption_enabled ? 'Enabled' : 'Disabled'}
        </span>
      ),
    },
    {
      header: 'Created',
      cell: (item) => formatDate(item.created_at),
    },
    {
      header: 'Actions',
      cell: (item) => (
        <Button variant="ghost" size="sm" onClick={() => void deleteBucket(item.name)}>
          Delete
        </Button>
      ),
    },
  ];

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Storage</h1>
          <p className={styles.subtitle}>Manage S3-compatible buckets with versioning and encryption metadata.</p>
        </div>
        <div className={styles.headerActions}>
          <Button variant="secondary" onClick={() => void loadBuckets()} loading={isLoading}>
            <RefreshCw size={16} /> Refresh
          </Button>
        </div>
      </header>

      {!hasCredentials ? (
        <div className={styles.notice}>
          <div>
            <strong>Storage API access is not configured.</strong>
            <p className={styles.noticeText}>Open Settings and add API credentials to load your buckets.</p>
          </div>
          <Link href="/settings" className="linkAccent">
            Go to Settings
          </Link>
        </div>
      ) : null}

      {error ? <div className={styles.error}>{error}</div> : null}

      <Card title="Create Bucket" subtitle="Persist object storage instantly" className={styles.panel}>
        <div className={styles.formRow}>
          <div className={styles.field}>
            <label htmlFor="bucketName">Bucket Name</label>
            <input
              id="bucketName"
              className={styles.input}
              value={newBucketName}
              placeholder="e.g. media-assets"
              onChange={(event) => setNewBucketName(event.target.value)}
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="bucketVisibility">Visibility</label>
            <select
              id="bucketVisibility"
              className={styles.select}
              value={createPublic ? 'public' : 'private'}
              onChange={(event) => setCreatePublic(event.target.value === 'public')}
            >
              <option value="private">Private</option>
              <option value="public">Public</option>
            </select>
          </div>
          <div className={styles.field}>
            <span className={styles.panelMeta}>Action</span>
            <Button onClick={() => void createBucket()} loading={isSaving}>
              <Plus size={16} /> Create Bucket
            </Button>
          </div>
        </div>
      </Card>

      <Card title="Bucket Inventory" subtitle="Live results from /storage/buckets" className={styles.panel}>
        <Table
          data={buckets}
          columns={columns}
          emptyMessage={isLoading ? 'Loading buckets...' : 'No buckets were returned by the API.'}
        />
      </Card>
    </div>
  );
}
