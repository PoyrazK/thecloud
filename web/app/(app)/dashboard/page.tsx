
'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { Activity, HardDrive, Network, RefreshCw, Server } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Button } from '@/components/ui/Button';
import { cloudApiRequest } from '@/lib/api';
import { eventStatus } from '@/lib/events';
import { useApiConfig } from '@/hooks/useApiConfig';
import styles from '../pages.module.css';

interface ApiInstance {
  id: string;
  status: string;
}

interface ApiBucket {
  id: string;
}

interface ApiVpc {
  id: string;
}

interface ApiEvent {
  id: string;
  action: string;
  resource_id: string;
  resource_type: string;
  created_at: string;
}

function relativeTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  const diff = Date.now() - date.getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export default function DashboardPage() {
  const { config, ready, hasCredentials } = useApiConfig();
  const [instances, setInstances] = useState<ApiInstance[]>([]);
  const [buckets, setBuckets] = useState<ApiBucket[]>([]);
  const [vpcs, setVpcs] = useState<ApiVpc[]>([]);
  const [events, setEvents] = useState<ApiEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadDashboard = useCallback(async () => {
    if (!ready) {
      return;
    }

    if (!hasCredentials) {
      setLoading(false);
      setInstances([]);
      setBuckets([]);
      setVpcs([]);
      setEvents([]);
      setError(null);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const [instanceData, bucketData, vpcData, eventData] = await Promise.all([
        cloudApiRequest<ApiInstance[]>('/instances', undefined, config),
        cloudApiRequest<ApiBucket[]>('/storage/buckets', undefined, config),
        cloudApiRequest<ApiVpc[]>('/vpcs', undefined, config),
        cloudApiRequest<ApiEvent[]>('/events?limit=5', undefined, config),
      ]);

      setInstances(instanceData ?? []);
      setBuckets(bucketData ?? []);
      setVpcs(vpcData ?? []);
      setEvents(eventData ?? []);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load dashboard data.';
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [config, hasCredentials, ready]);

  useEffect(() => {
    void loadDashboard();
  }, [loadDashboard]);

  const runningInstances = useMemo(
    () => instances.filter((item) => item.status?.toLowerCase().includes('running')).length,
    [instances]
  );

  const healthyEventRatio = useMemo(() => {
    if (events.length === 0) {
      return 'n/a';
    }

    const healthyEvents = events.filter((event) => eventStatus(event.action) === 'success').length;
    const ratio = Math.round((healthyEvents / events.length) * 100);
    return `${ratio}%`;
  }, [events]);

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Dashboard</h1>
          <p className={styles.subtitle}>Live control plane summary across compute, storage, and network.</p>
        </div>
        <div className={styles.headerActions}>
          <Button variant="secondary" onClick={() => void loadDashboard()} loading={loading}>
            <RefreshCw size={15} />
            Refresh
          </Button>
        </div>
      </header>

      {!hasCredentials ? (
        <div className={styles.notice}>
          <div>
            <strong>API access not configured.</strong>
            <p className={styles.noticeText}>Add your API key and optional tenant ID in Settings to load live data.</p>
          </div>
          <Link href="/settings" className="linkAccent">
            Open Settings
          </Link>
        </div>
      ) : null}

      {error ? <div className={styles.error}>{error}</div> : null}

      <section className={styles.statsGrid}>
        <Link href="/compute" className={styles.statLink}>
          <article className={styles.stat}>
            <div className={styles.statLabel}>Instances</div>
            <div className={styles.statValue}>{runningInstances}</div>
            <div className={styles.statHint}>Running now</div>
          </article>
        </Link>
        <Link href="/storage" className={styles.statLink}>
          <article className={styles.stat}>
            <div className={styles.statLabel}>Storage Buckets</div>
            <div className={styles.statValue}>{buckets.length}</div>
            <div className={styles.statHint}>Object stores</div>
          </article>
        </Link>
        <Link href="/network" className={styles.statLink}>
          <article className={styles.stat}>
            <div className={styles.statLabel}>VPC Networks</div>
            <div className={styles.statValue}>{vpcs.length}</div>
            <div className={styles.statHint}>Isolated network spaces</div>
          </article>
        </Link>
        <Link href="/activity" className={styles.statLink}>
          <article className={styles.stat}>
            <div className={styles.statLabel}>Recent Event Health</div>
            <div className={styles.statValue}>{healthyEventRatio}</div>
            <div className={styles.statHint}>From latest control-plane events</div>
          </article>
        </Link>
      </section>

      <section className={styles.gridTwo}>
        <Card title="Recent Activity" subtitle="Latest control-plane events" className={styles.panel}>
          {events.length === 0 ? (
            <div className={styles.empty}>{loading ? 'Loading events...' : 'No recent events found.'}</div>
          ) : (
            <div className={styles.activityList}>
              {events.map((event) => (
                <div key={event.id} className={styles.activityItem}>
                  <div className={styles.activityTop}>
                    <span>{event.action}</span>
                    <StatusIndicator status={eventStatus(event.action)} label={relativeTime(event.created_at)} />
                  </div>
                  <div className={styles.activityMeta}>
                    {event.resource_type}: {event.resource_id}
                  </div>
                </div>
              ))}
            </div>
          )}
        </Card>

        <Card title="Quick Actions" subtitle="Jump into managed surfaces" className={styles.panel}>
          <div className={styles.infoList}>
            <Link href="/compute" className={styles.infoRowLink}>
              <span className={styles.infoKey}>Compute</span>
              <span className={styles.infoRowOpen}>
                Open <Server size={14} style={{ verticalAlign: 'text-bottom' }} />
              </span>
            </Link>
            <Link href="/storage" className={styles.infoRowLink}>
              <span className={styles.infoKey}>Storage</span>
              <span className={styles.infoRowOpen}>
                Open <HardDrive size={14} style={{ verticalAlign: 'text-bottom' }} />
              </span>
            </Link>
            <Link href="/network" className={styles.infoRowLink}>
              <span className={styles.infoKey}>Network</span>
              <span className={styles.infoRowOpen}>
                Open <Network size={14} style={{ verticalAlign: 'text-bottom' }} />
              </span>
            </Link>
            <Link href="/activity" className={styles.infoRowLink}>
              <span className={styles.infoKey}>Activity</span>
              <span className={styles.infoRowOpen}>
                Open <Activity size={14} style={{ verticalAlign: 'text-bottom' }} />
              </span>
            </Link>
          </div>
        </Card>
      </section>
    </div>
  );
}
