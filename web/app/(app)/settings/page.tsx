
'use client';

import { useEffect, useMemo, useState } from 'react';
import { KeyRound, Link2, Server, UserRound } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { cloudApiRequest, type CloudApiConfig } from '@/lib/api';
import { useApiConfig } from '@/hooks/useApiConfig';
import styles from '../pages.module.css';

interface Profile {
  id?: string;
  email?: string;
  name?: string;
}

interface Tenant {
  id: string;
  name: string;
  slug?: string;
}

export default function SettingsPage() {
  const { config, ready, updateConfig } = useApiConfig();

  const [baseUrl, setBaseUrl] = useState('http://localhost:8080');
  const [apiKey, setApiKey] = useState('');
  const [tenantId, setTenantId] = useState('');
  const [showKey, setShowKey] = useState(false);

  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [profile, setProfile] = useState<Profile | null>(null);
  const [tenants, setTenants] = useState<Tenant[]>([]);

  useEffect(() => {
    if (!ready) return;
    setBaseUrl(config.baseUrl);
    setApiKey(config.apiKey);
    setTenantId(config.tenantId);
  }, [config, ready]);

  const maskedKey = useMemo(() => {
    if (!apiKey) return 'Not configured';
    if (apiKey.length < 10) return `${apiKey.slice(0, 2)}****`;
    return `${apiKey.slice(0, 6)}...${apiKey.slice(-4)}`;
  }, [apiKey]);

  const saveSettings = () => {
    setSaving(true);
    setError(null);
    setMessage(null);

    try {
      updateConfig({
        baseUrl,
        apiKey,
        tenantId,
      });
      setMessage('Connection settings saved locally in your browser.');
    } finally {
      setSaving(false);
    }
  };

  const testConnection = async () => {
    const candidate: CloudApiConfig = {
      baseUrl,
      apiKey,
      tenantId,
    };

    setTesting(true);
    setError(null);
    setMessage(null);
    setProfile(null);
    setTenants([]);

    try {
      const me = await cloudApiRequest<Profile>('/auth/me', undefined, candidate);
      let tenantWarning: string | null = null;
      let tenantData: Tenant[] = [];
      try {
        tenantData = await cloudApiRequest<Tenant[]>('/tenants', undefined, candidate);
      } catch (tenantErr) {
        const tenantReason = tenantErr instanceof Error ? tenantErr.message : 'Tenant list unavailable.';
        tenantWarning = `Authentication succeeded, but loading tenants failed: ${tenantReason}`;
      }

      setProfile(me ?? null);
      setTenants(tenantData ?? []);
      if (!tenantWarning) {
        setMessage('Connection successful. Credentials and endpoint are valid.');
      } else {
        setError(tenantWarning);
      }
    } catch (err) {
      const reason = err instanceof Error ? err.message : 'Connection test failed.';
      setProfile(null);
      setTenants([]);
      setError(reason);
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Settings</h1>
          <p className={styles.subtitle}>Configure API endpoint, credentials, and tenant scope for console data access.</p>
        </div>
        <div className={styles.headerActions}>
          <Button variant="secondary" onClick={() => setShowKey((current) => !current)}>
            {showKey ? 'Hide Key' : 'Show Key'}
          </Button>
        </div>
      </header>

      {message ? <div className={styles.notice}><strong>Status:</strong> <span className={styles.noticeText}>{message}</span></div> : null}
      {error ? <div className={styles.error}>{error}</div> : null}

      <Card title="API Connection" subtitle="Used by compute, storage, network, and activity pages" className={styles.panel}>
        <div className={styles.formRow}>
          <div className={styles.field}>
            <label htmlFor="baseUrl">Base URL</label>
            <input
              id="baseUrl"
              className={styles.input}
              value={baseUrl}
              onChange={(event) => setBaseUrl(event.target.value)}
              placeholder="http://localhost:8080"
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="apiKey">API Key</label>
            <input
              id="apiKey"
              className={styles.input}
              type={showKey ? 'text' : 'password'}
              value={apiKey}
              onChange={(event) => setApiKey(event.target.value)}
              placeholder="thecloud_xxxxx"
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="tenantId">Tenant ID (Optional)</label>
            <input
              id="tenantId"
              className={styles.input}
              value={tenantId}
              onChange={(event) => setTenantId(event.target.value)}
              placeholder="uuid"
            />
          </div>
        </div>

        <div className={styles.formActions}>
          <Button onClick={() => void saveSettings()} loading={saving}>Save Settings</Button>
          <Button variant="secondary" onClick={() => void testConnection()} loading={testing}>
            Test Connection
          </Button>
        </div>
      </Card>

      <section className={styles.gridTwo}>
        <Card title="Connection Snapshot" className={styles.panel}>
          <div className={styles.infoList}>
            <div className={styles.infoRow}>
              <span className={styles.infoKey}><Server size={14} style={{ verticalAlign: 'text-bottom' }} /> Endpoint</span>
              <span className={styles.infoValue}>{baseUrl || 'Not set'}</span>
            </div>
            <div className={styles.infoRow}>
              <span className={styles.infoKey}><KeyRound size={14} style={{ verticalAlign: 'text-bottom' }} /> API Key</span>
              <span className={styles.infoValue}>{maskedKey}</span>
            </div>
            <div className={styles.infoRow}>
              <span className={styles.infoKey}><Link2 size={14} style={{ verticalAlign: 'text-bottom' }} /> Tenant Header</span>
              <span className={styles.infoValue}>{tenantId || 'None (default tenant)'}</span>
            </div>
          </div>
        </Card>

        <Card title="Identity Preview" className={styles.panel}>
          <div className={styles.infoList}>
            <div className={styles.infoRow}>
              <span className={styles.infoKey}><UserRound size={14} style={{ verticalAlign: 'text-bottom' }} /> User</span>
              <span className={styles.infoValue}>{profile?.name || 'Unknown'}</span>
            </div>
            <div className={styles.infoRow}>
              <span className={styles.infoKey}>Email</span>
              <span className={styles.infoValue}>{profile?.email || 'Unavailable'}</span>
            </div>
            <div className={styles.infoRow}>
              <span className={styles.infoKey}>Tenant Memberships</span>
              <span className={styles.infoValue}>{tenants.length}</span>
            </div>
          </div>
        </Card>
      </section>

      <Card title="Tenant Memberships" subtitle="Returned by /tenants" className={styles.panel}>
        {tenants.length === 0 ? (
          <div className={styles.empty}>No tenant records loaded yet. Run Test Connection to fetch memberships.</div>
        ) : (
          <div className={styles.activityList}>
            {tenants.map((tenant) => (
              <div key={tenant.id} className={styles.activityItem}>
                <div className={styles.activityTop}>
                  <span>{tenant.name}</span>
                  <span className={styles.badge + ' ' + styles.badgeNeutral}>{tenant.slug || 'no-slug'}</span>
                </div>
                <div className={styles.activityMeta}>{tenant.id}</div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
