
import React from 'react';
import styles from './StatusIndicator.module.css';

interface StatusIndicatorProps {
  status: 'running' | 'stopped' | 'pending' | 'error' | 'success' | 'failure';
  label?: string;
}

export const StatusIndicator: React.FC<StatusIndicatorProps> = ({ status, label }) => {
  return (
    <div className={styles.container}>
      <span className={`${styles.dot} ${styles[status]}`} />
      {label && <span className={styles.label}>{label}</span>}
    </div>
  );
};
