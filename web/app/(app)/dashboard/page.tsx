
'use client';

import { useEffect, useState } from 'react';
import { Card } from '@/components/ui/Card';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Button } from '@/components/ui/Button';
import Link from 'next/link';
import { apiGet } from '@/lib/api';
import { formatDateTime } from '@/lib/format';
import { Activity, Server, HardDrive, Cpu } from 'lucide-react';

interface Summary {
  total_instances: number;
  running_instances: number;
  stopped_instances: number;
  total_volumes: number;
  attached_volumes: number;
  total_vpcs: number;
  total_storage_mb: number;
}

interface Event {
  id: string;
  action: string;
  resource_id: string;
  resource_type: string;
  created_at: string;
}

interface DashboardStats {
  summary: Summary;
  recent_events: Event[];
}

const emptySummary: Summary = {
  total_instances: 0,
  running_instances: 0,
  stopped_instances: 0,
  total_volumes: 0,
  attached_volumes: 0,
  total_vpcs: 0,
  total_storage_mb: 0,
};

export default function Home() {
  const [summary, setSummary] = useState<Summary>(emptySummary);
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const loadDashboard = async () => {
    setLoading(true);
    setError('');
    try {
      const data = await apiGet<DashboardStats>('/api/dashboard/stats');
      setSummary(data.summary);
      setEvents(data.recent_events || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load dashboard');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadDashboard();
  }, []);

  const healthyPercent = summary.total_instances
    ? ((summary.running_instances / summary.total_instances) * 100).toFixed(1)
    : '0.0';

  const storageGB = (summary.total_storage_mb / 1024).toFixed(1);

  return (
    <div style={{ maxWidth: '1280px', margin: '0 auto' }}>
      <header style={{ 
        marginBottom: '32px', 
        paddingTop: '20px',
        position: 'sticky',
        top: 0,
        zIndex: 10
      }}>
        <h1 style={{ 
          fontSize: '34px', 
          fontWeight: 700, 
          marginBottom: '4px',
          letterSpacing: '0.01em',
          color: 'var(--text-primary)'
        }}>
          Dashboard
        </h1>
        <p style={{ color: 'var(--text-secondary)', fontSize: '15px' }}>Overview</p>
      </header>
  
      {/* Metrics Grid */}
      <div style={{ 
        display: 'grid', 
        gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', 
        gap: '20px',
        marginBottom: '32px'
      }}>
        <Card>
          <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
            <div style={{ 
              padding: '12px', 
              borderRadius: '50%', /* Circular icon backgrounds */
              background: 'rgba(10, 132, 255, 0.15)',
              color: 'var(--accent-blue)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: '48px',
              height: '48px'
            }}>
              <Server size={22} strokeWidth={2.5} />
            </div>
            <div>
              <div style={{ fontSize: '26px', fontWeight: 600, lineHeight: 1 }}>{summary.running_instances}</div>
              <div style={{ color: 'var(--text-secondary)', fontSize: '13px', marginTop: '4px', fontWeight: 500 }}>Active Instances</div>
            </div>
          </div>
        </Card>
        
        <Card>
          <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
            <div style={{ 
              padding: '12px', 
               borderRadius: '50%',
              background: 'rgba(48, 209, 88, 0.15)',
              color: 'var(--accent-green)',
               display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: '48px',
              height: '48px'
            }}>
              <Activity size={22} strokeWidth={2.5} />
            </div>
            <div>
              <div style={{ fontSize: '26px', fontWeight: 600, lineHeight: 1 }}>{healthyPercent}%</div>
               <div style={{ color: 'var(--text-secondary)', fontSize: '13px', marginTop: '4px', fontWeight: 500 }}>Running vs Total</div>
            </div>
          </div>
        </Card>

        <Card>
          <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
            <div style={{ 
              padding: '12px', 
               borderRadius: '50%',
              background: 'rgba(255, 159, 10, 0.15)',
              color: 'var(--accent-orange)',
               display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: '48px',
              height: '48px'
            }}>
              <HardDrive size={22} strokeWidth={2.5} />
            </div>
            <div>
              <div style={{ fontSize: '26px', fontWeight: 600, lineHeight: 1 }}>{storageGB} GB</div>
               <div style={{ color: 'var(--text-secondary)', fontSize: '13px', marginTop: '4px', fontWeight: 500 }}>Storage Provisioned</div>
            </div>
          </div>
        </Card>
        
        <Card>
           <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
           <div style={{ 
              padding: '12px', 
               borderRadius: '50%',
              background: 'rgba(255, 69, 58, 0.15)',
              color: 'var(--accent-red)',
               display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: '48px',
              height: '48px'
            }}>
              <Cpu size={22} strokeWidth={2.5} />
            </div>
            <div>
              <div style={{ fontSize: '26px', fontWeight: 600, lineHeight: 1 }}>{summary.total_vpcs}</div>
               <div style={{ color: 'var(--text-secondary)', fontSize: '13px', marginTop: '4px', fontWeight: 500 }}>VPCs</div>
            </div>
          </div>
        </Card>
      </div>

      {error ? (
        <div style={{ marginBottom: '16px', color: 'var(--accent-red)' }}>{error}</div>
      ) : null}

      {/* Recent Activity & Resources */}
      <div style={{ 
        display: 'grid', 
        gridTemplateColumns: '2fr 1fr', 
        gap: '24px' 
      }}>
        <Card title="Recent Activity" style={{ minHeight: '400px' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            {events.length === 0 && !loading ? (
              <div style={{ color: 'var(--text-secondary)' }}>No recent events.</div>
            ) : null}
            {events.map((event) => (
              <div key={event.id} style={{ 
                display: 'flex', 
                alignItems: 'center', 
                justifyContent: 'space-between',
                padding: '12px 0',
                borderBottom: '1px solid var(--glass-border)'
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                   <StatusIndicator status="running" />
                   <div>
                     <div style={{ fontWeight: 500 }}>{event.action}</div>
                     <div style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
                       {event.resource_type} {event.resource_id}
                     </div>
                     <div style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
                       {formatDateTime(event.created_at)}
                     </div>
                   </div>
                </div>
                <Button variant="ghost" size="sm">View</Button>
              </div>
            ))}
          </div>
        </Card>

        <Card title="Quick Actions">
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <Link href="/compute">
              <Button variant="primary" style={{ width: '100%' }}>Launch Instance</Button>
            </Link>
            <Link href="/storage">
              <Button variant="secondary" style={{ width: '100%' }}>Create Bucket</Button>
            </Link>
            <Link href="/activity">
               <Button variant="ghost" style={{ width: '100%' }}>View Activity</Button>
            </Link>
          </div>
        </Card>
      </div>
    </div>
  );
}
