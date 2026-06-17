
'use client';

import React, { useState } from 'react';
import { X } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import styles from './LaunchInstanceModal.module.css';

interface LaunchInstanceData {
  name: string;
  image: string;
  ports: string;
  vpcId?: string;
}

interface VpcOption {
  id: string;
  name: string;
  cidr_block?: string;
}

interface LaunchInstanceModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (data: LaunchInstanceData) => Promise<void>;
  isSubmitting?: boolean;
  vpcs: VpcOption[];
}

export const LaunchInstanceModal: React.FC<LaunchInstanceModalProps> = ({
  isOpen,
  onClose,
  onSubmit,
  isSubmitting = false,
  vpcs,
}) => {
  const [formData, setFormData] = useState<LaunchInstanceData>({
    name: '',
    image: 'ubuntu-22.04',
    ports: '',
    vpcId: undefined,
  });
  const [localError, setLocalError] = useState<string | null>(null);

  const handleClose = () => {
    setLocalError(null);
    onClose();
  };

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!formData.name.trim()) {
      setLocalError('Instance name is required.');
      return;
    }

    setLocalError(null);

    try {
      await onSubmit({
        ...formData,
        name: formData.name.trim(),
        ports: formData.ports.trim(),
      });
      handleClose();
      setFormData({ 
        name: '',
        image: 'ubuntu-22.04',
        ports: '',
        vpcId: undefined,
      });
    } catch {
      // Parent component shows the API error in page-level banner.
    }
  };

  return (
    <div className={styles.overlay}>
      <div className={`${styles.modal} material-platter`}>
        <div className={styles.header}>
          <h2 className={styles.title}>Launch Instance</h2>
          <Button variant="ghost" onClick={handleClose} className={styles.closeBtn} disabled={isSubmitting}>
            <X size={18} />
          </Button>
        </div>
        
        <form onSubmit={handleSubmit} className={styles.form}>
          <div className={styles.field}>
            <label className={styles.label}>Name</label>
            <input 
              type="text" 
              className={styles.input} 
              placeholder="e.g. web-server-01"
              value={formData.name}
              onChange={(e) => setFormData({...formData, name: e.target.value})}
              autoFocus
            />
          </div>

          <div className={styles.row}>
            <div className={styles.field}>
              <label className={styles.label}>Image</label>
              <select 
                className={styles.select}
                value={formData.image}
                onChange={(e) => setFormData({...formData, image: e.target.value})}
              >
                <option value="ubuntu-22.04">Ubuntu 22.04 LTS</option>
                <option value="alpine-3.18">Alpine Linux 3.18</option>
                <option value="nginx-latest">Nginx (Latest)</option>
                <option value="postgres-15">PostgreSQL 15</option>
              </select>
            </div>

             <div className={styles.field}>
              <label className={styles.label}>VPC Network (Optional)</label>
              <select 
                className={styles.select}
                value={formData.vpcId ?? ''}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    vpcId: e.target.value ? e.target.value : undefined,
                  })
                }
              >
                <option value="">No VPC</option>
                {vpcs.map((vpc) => (
                  <option key={vpc.id} value={vpc.id}>
                    {vpc.name} {vpc.cidr_block ? `(${vpc.cidr_block})` : ''}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className={styles.field}>
            <label className={styles.label}>Port Mapping (Host:Container)</label>
            <input 
              type="text" 
              className={styles.input} 
              placeholder="e.g. 8080:80, 8443:443"
              value={formData.ports}
              onChange={(e) => setFormData({...formData, ports: e.target.value})}
            />
            <p className={styles.helpText}>Comma-separated host:container mappings. Leave empty for internal-only instance.</p>
          </div>

          {localError ? <div className={styles.error}>{localError}</div> : null}

          <div className={styles.footer}>
            <Button type="button" variant="secondary" onClick={handleClose} disabled={isSubmitting}>Cancel</Button>
            <Button type="submit" variant="primary" loading={isSubmitting}>
              Launch Instance
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
