'use client';

import React from 'react';
import styles from './Table.module.css';

export interface Column<T> {
  header: string;
  accessorKey?: keyof T;
  cell?: (item: T) => React.ReactNode;
  width?: string;
}

interface TableProps<T> {
  data: T[];
  columns: Column<T>[];
  onRowClick?: (item: T) => void;
  emptyMessage?: string;
  getRowKey?: (item: T, index: number) => React.Key;
}

export function Table<T>({
  data,
  columns,
  onRowClick,
  emptyMessage = 'No data found.',
  getRowKey,
}: TableProps<T>) {
  return (
    <div className={styles.container}>
      <table className={styles.table}>
        <thead>
          <tr>
            {columns.map((col, i) => (
              <th key={i} style={{ width: col.width }}>
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className={styles.emptyCell}>
                {emptyMessage}
              </td>
            </tr>
          ) : null}
          {data.map((item, rowIndex) => (
            <tr
              key={
                getRowKey
                  ? getRowKey(item, rowIndex)
                  : typeof item === 'object' && item !== null && 'id' in (item as Record<string, unknown>)
                    ? String((item as Record<string, unknown>).id)
                    : rowIndex
              }
              onClick={() => onRowClick && onRowClick(item)}
              className={onRowClick ? styles.clickable : ''}
            >
              {columns.map((col, colIndex) => (
                <td key={colIndex}>
                  {col.cell
                    ? col.cell(item)
                    : col.accessorKey
                      ? (item[col.accessorKey] == null ? '' : String(item[col.accessorKey]))
                      : ''}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
