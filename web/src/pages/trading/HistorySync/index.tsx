import {
  DatabaseOutlined,
  HistoryOutlined,
  ReloadOutlined,
  TagsOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Drawer, message, Space, Tag, Tooltip, Typography } from 'antd';
import React, { useRef, useState } from 'react';
import {
  fetchHistorySyncStatuses,
  syncExchange,
} from '@/services/trading/catalog';
import HistoryBackfillForm from '../Configs/components/HistoryBackfillForm';
import HistorySymbolsForm from './components/HistorySymbolsForm';
import { fetchUserStreamEvents, fetchUserStreamStates } from '@/services/trading/observation';

const coverageMeta = {
  healthy: { color: 'success', label: '完整' },
  pending: { color: 'processing', label: '待首次同步' },
  stale: { color: 'warning', label: '同步滞后' },
  incomplete: { color: 'warning', label: '覆盖不完整' },
  error: { color: 'error', label: '同步失败' },
  disabled: { color: 'default', label: '已停用' },
} as const;

const HistorySync: React.FC = () => {
  const actionRef = useRef<ActionType>(null);
  const streamActionRef = useRef<ActionType>(null);
  const [symbolsStatus, setSymbolsStatus] = useState<TradingAPI.HistorySyncStatus>();
  const [backfillStatus, setBackfillStatus] = useState<TradingAPI.HistorySyncStatus>();
  const [syncingID, setSyncingID] = useState<string>();
  const [eventExchangeID, setEventExchangeID] = useState<string>();

  const handleSync = async (record: TradingAPI.HistorySyncStatus) => {
    if (syncingID) return;
    setSyncingID(record.exchange_id);
    const key = `history-sync-${record.exchange_id}`;
    message.loading({ key, content: '正在同步账户与历史数据...', duration: 0 });
    try {
      const response = await syncExchange(record.exchange_id);
      const data = response.data;
      message.success({
        key,
        content: data
          ? `同步完成：订单 ${data.order_count}、成交 ${data.fill_count}、收益事件 ${data.income_event_count}`
          : '同步完成',
        duration: 6,
      });
      actionRef.current?.reload();
      streamActionRef.current?.reload();
    } catch (error) {
      message.error({ key, content: error instanceof Error ? error.message : '同步失败', duration: 8 });
    } finally {
      setSyncingID(undefined);
    }
  };

  const columns: ProColumns<TradingAPI.HistorySyncStatus>[] = [
    {
      title: '交易所账户',
      width: 190,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.exchange_name}</Typography.Text>
          <Typography.Text type="secondary">
            {record.exchange_type.toUpperCase()} · {record.account_type}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: '完整性',
      dataIndex: 'coverage_status',
      width: 120,
      render: (_, record) => {
        const meta = coverageMeta[record.coverage_status] ?? coverageMeta.pending;
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    {
      title: '注册交易对',
      dataIndex: 'registered_symbols',
      width: 220,
      render: (_, record) =>
        record.registered_symbols.length ? (
          <Tooltip title={record.registered_symbols.join(', ')}>
            <Typography.Text ellipsis>{record.registered_symbols.join(', ')}</Typography.Text>
          </Tooltip>
        ) : (
          '--'
        ),
    },
    { title: '订单', dataIndex: 'order_count', width: 88, align: 'right' },
    { title: '成交', dataIndex: 'fill_count', width: 88, align: 'right' },
    { title: '收益事件', dataIndex: 'income_event_count', width: 100, align: 'right' },
    { title: '完整交易', dataIndex: 'closed_trade_count', width: 100, align: 'right' },
    {
      title: '关联缺口',
      dataIndex: 'gap_count',
      width: 100,
      align: 'right',
      render: (_, record) =>
        record.gap_count > 0 ? (
          <Tooltip title={record.gap_details.join('；')}>
            <Typography.Text type="danger">{record.gap_count}</Typography.Text>
          </Tooltip>
        ) : (
          '0'
        ),
    },
    {
      title: '同步到',
      dataIndex: 'synced_until',
      valueType: 'dateTime',
      width: 180,
      renderText: (value) => value || '--',
    },
    {
      title: '状态说明',
      dataIndex: 'warning',
      width: 280,
      ellipsis: true,
      render: (_, record) => (
        <Typography.Text type={record.coverage_status === 'error' ? 'danger' : 'secondary'}>
          {record.error_message || record.warning || '同步正常'}
        </Typography.Text>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 150,
      render: (_, record) => (
        <Space size={4}>
          <Tooltip title="立即同步">
            <Button
              type="text"
              icon={<ReloadOutlined />}
              loading={syncingID === record.exchange_id}
              disabled={!record.enabled || Boolean(syncingID)}
              onClick={() => void handleSync(record)}
            />
          </Tooltip>
          {record.exchange_type === 'binance' && (
            <Tooltip title="维护历史交易对">
              <Button type="text" icon={<TagsOutlined />} onClick={() => setSymbolsStatus(record)} />
            </Tooltip>
          )}
          {record.exchange_type !== 'bybit' && (
            <Tooltip title="区间回填">
              <Button type="text" icon={<HistoryOutlined />} onClick={() => setBackfillStatus(record)} />
            </Tooltip>
          )}
        </Space>
      ),
    },
  ];

  const streamColumns: ProColumns<TradingAPI.UserStreamState>[] = [
    { title: '交易所', dataIndex: 'exchange_type', width: 110, renderText: (value) => String(value).toUpperCase() },
    { title: '账户 ID', dataIndex: 'exchange_id', width: 220, ellipsis: true },
    {
      title: '连接状态', dataIndex: 'status', width: 120,
      render: (_, record) => <Tag color={record.status === 'connected' ? 'success' : record.status === 'reconnecting' ? 'warning' : 'default'}>{record.status}</Tag>,
    },
    { title: '重连次数', dataIndex: 'reconnect_count', width: 100, align: 'right' },
    { title: '最近事件', dataIndex: 'last_event_at', valueType: 'dateTime', width: 180, renderText: (value) => value || '--' },
    { title: '最近对账', dataIndex: 'last_reconciled_at', valueType: 'dateTime', width: 180, renderText: (value) => value || '--' },
    { title: '错误', dataIndex: 'last_error', width: 260, ellipsis: true, renderText: (value) => value || '--' },
    {
      title: '操作', valueType: 'option', width: 90,
      render: (_, record) => <Button type="link" size="small" onClick={() => setEventExchangeID(record.exchange_id)}>事件</Button>,
    },
  ];

  return (
    <PageContainer>
      <Alert
        showIcon
        icon={<DatabaseOutlined />}
        type="info"
        message="完整性状态根据增量游标、最近同步时间、Binance 历史交易对覆盖，以及订单与成交关联缺口计算。异常或滞后账户应先回补后再做收益分析。"
        style={{ marginBottom: 16 }}
      />
      <ProTable<TradingAPI.UserStreamState>
        actionRef={streamActionRef}
        headerTitle={<Space><ThunderboltOutlined />实时用户流</Space>}
        rowKey="exchange_id"
        columns={streamColumns}
        search={false}
        request={async () => {
          const response = await fetchUserStreamStates();
          const data = response.data ?? [];
          return { data, success: true, total: data.length };
        }}
        pagination={false}
        scroll={{ x: 1260 }}
        options={{ density: true, reload: true }}
        locale={{ emptyText: '用户流未启用或尚未建立连接' }}
        style={{ marginBottom: 16 }}
      />
      <ProTable<TradingAPI.HistorySyncStatus>
        actionRef={actionRef}
        rowKey="exchange_id"
        columns={columns}
        search={false}
        request={async () => {
          const response = await fetchHistorySyncStatuses();
          const data = response.data ?? [];
          return { data, success: true, total: data.length };
        }}
        pagination={false}
        scroll={{ x: 1510 }}
        options={{ density: true, reload: true, fullScreen: true }}
      />
      <Drawer
        open={Boolean(eventExchangeID)}
        width={820}
        title="用户流事件审计"
        onClose={() => setEventExchangeID(undefined)}
        destroyOnClose
      >
        <ProTable<TradingAPI.UserStreamEvent>
          rowKey="id"
          search={false}
          columns={[
            { title: '事件类型', dataIndex: 'event_type', width: 180 },
            { title: '事件时间', dataIndex: 'event_time', valueType: 'dateTime', width: 180 },
            { title: '接收时间', dataIndex: 'received_at', valueType: 'dateTime', width: 180 },
            { title: '载荷哈希', dataIndex: 'payload_hash', ellipsis: true },
          ]}
          params={{ exchangeID: eventExchangeID }}
          request={fetchUserStreamEvents}
          pagination={{ pageSize: 10 }}
          options={{ density: true, reload: true }}
        />
      </Drawer>
      <HistorySymbolsForm
        visible={Boolean(symbolsStatus)}
        status={symbolsStatus}
        onCancel={() => setSymbolsStatus(undefined)}
        onSuccess={() => {
          setSymbolsStatus(undefined);
          actionRef.current?.reload();
        }}
      />
      <HistoryBackfillForm
        visible={Boolean(backfillStatus)}
        exchange={
          backfillStatus
            ? {
                id: backfillStatus.exchange_id,
                name: backfillStatus.exchange_name,
                exchange_type: backfillStatus.exchange_type,
              }
            : undefined
        }
        onCancel={() => setBackfillStatus(undefined)}
        onSuccess={() => {
          setBackfillStatus(undefined);
          actionRef.current?.reload();
        }}
      />
    </PageContainer>
  );
};

export default HistorySync;
