import { AuditOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import { Alert, Descriptions, Drawer, Space, Tag, Timeline, Typography } from 'antd';
import React, { useEffect, useState } from 'react';
import {
  fetchTradeCommandEvents,
  fetchTradeCommands,
  fetchTradingSafetyState,
} from '@/services/trading/execution';
import { formatDecimal } from '@/utils/decimal';

const statusMeta: Record<string, { color: string; label: string }> = {
  validated: { color: 'processing', label: '已校验' },
  simulated: { color: 'default', label: '模拟完成' },
  submitted: { color: 'warning', label: '已提交' },
  accepted: { color: 'processing', label: '已受理' },
  partially_filled: { color: 'warning', label: '部分成交' },
  filled: { color: 'success', label: '已成交' },
  rejected: { color: 'error', label: '已拒绝' },
  cancelled: { color: 'default', label: '已撤销' },
};

const Execution: React.FC = () => {
  const [safety, setSafety] = useState<TradingAPI.TradingSafetyState>();
  const [events, setEvents] = useState<TradingAPI.TradeCommandEvent[]>([]);
  const [eventLoading, setEventLoading] = useState(false);
  const [eventOpen, setEventOpen] = useState(false);

  useEffect(() => {
    fetchTradingSafetyState()
      .then((response) => setSafety(response.data))
      .catch(() => setSafety(undefined));
  }, []);

  const showEvents = async (id: string) => {
    setEventOpen(true);
    setEventLoading(true);
    try {
      const response = await fetchTradeCommandEvents(id);
      setEvents(response.data ?? []);
    } finally {
      setEventLoading(false);
    }
  };

  const columns: ProColumns<TradingAPI.TradeCommand>[] = [
    { title: 'Trader ID', dataIndex: 'trader_id', key: 'traderID', ellipsis: true, width: 170 },
    { title: '交易对', dataIndex: 'symbol', key: 'symbol', width: 120 },
    {
      title: '动作',
      dataIndex: 'action',
      search: false,
      width: 90,
      render: (_, record) => <Tag>{record.action.toUpperCase()}</Tag>,
    },
    {
      title: '环境',
      search: false,
      width: 100,
      render: (_, record) => (
        <Tag color={record.simulation ? 'default' : record.testnet ? 'blue' : 'error'}>
          {record.simulation ? '模拟' : record.testnet ? '测试网' : '实盘'}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      valueType: 'select',
      valueEnum: Object.fromEntries(Object.entries(statusMeta).map(([key, value]) => [key, value.label])),
      width: 110,
      render: (_, record) => {
        const meta = statusMeta[record.status] ?? { color: 'default', label: record.status };
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    {
      title: '数量',
      dataIndex: 'quantity',
      search: false,
      align: 'right',
      width: 120,
      renderText: (value) => formatDecimal(value, { maximumFractionDigits: 12 }),
    },
    {
      title: '价格',
      dataIndex: 'price',
      search: false,
      align: 'right',
      width: 120,
      renderText: (value, record) =>
        record.order_type === 'market' || Number(value) === 0
          ? '市价'
          : formatDecimal(value, { maximumFractionDigits: 12 }),
    },
    {
      title: '成交数量',
      dataIndex: 'executed_quantity',
      search: false,
      align: 'right',
      width: 120,
      renderText: (value) => Number(value) > 0 ? formatDecimal(value, { maximumFractionDigits: 12 }) : '--',
    },
    {
      title: '成交均价',
      dataIndex: 'average_price',
      search: false,
      align: 'right',
      width: 120,
      renderText: (value) => Number(value) > 0 ? formatDecimal(value, { maximumFractionDigits: 12 }) : '--',
    },
    {
      title: '止损 / 止盈',
      search: false,
      align: 'right',
      width: 170,
      render: (_, record) => record.stop_loss || record.take_profit ? (
        <Space direction="vertical" size={0}>
          <Typography.Text type="danger">SL {formatDecimal(record.stop_loss, { maximumFractionDigits: 12 })}</Typography.Text>
          <Typography.Text type="success">TP {formatDecimal(record.take_profit, { maximumFractionDigits: 12 })}</Typography.Text>
        </Space>
      ) : '--',
    },
    {
      title: '错误',
      dataIndex: 'error_message',
      search: false,
      ellipsis: true,
      width: 220,
      render: (_, record) =>
        record.error_message ? <Typography.Text type="danger">{record.error_message}</Typography.Text> : '--',
    },
    { title: '创建时间', dataIndex: 'created_at', valueType: 'dateTime', search: false, width: 180 },
    {
      title: '操作',
      valueType: 'option',
      width: 90,
      render: (_, record) => (
        <Typography.Link onClick={() => void showEvents(record.id)}>审计</Typography.Link>
      ),
    },
  ];

  return (
    <PageContainer title="交易执行" subTitle="命令、风险校验与状态变更审计">
      <Alert
        showIcon
        icon={<SafetyCertificateOutlined />}
        type={safety?.live_enabled ? 'warning' : 'success'}
        message={safety?.live_enabled ? '用户级实盘开关已启用' : '用户级实盘开关未启用'}
        description="服务端总开关默认关闭；模拟命令不会调用交易 Provider。真实交易 Provider 尚未注册，因此当前系统不会向交易所提交订单。"
        style={{ marginBottom: 16 }}
      />
      <ProTable
        rowKey="id"
        columns={columns}
        request={fetchTradeCommands}
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 1730 }}
        options={{ density: true, reload: true, fullScreen: true }}
      />
      <Drawer
        title={<Space><AuditOutlined />命令审计</Space>}
        width={560}
        open={eventOpen}
        loading={eventLoading}
        onClose={() => setEventOpen(false)}
      >
        <Timeline
          items={events.map((event) => ({
            color: event.to_status === 'rejected' ? 'red' : 'blue',
            children: (
              <Descriptions column={1} size="small" bordered>
                <Descriptions.Item label="事件">{event.event_type}</Descriptions.Item>
                <Descriptions.Item label="状态">{event.from_status || '--'} → {event.to_status}</Descriptions.Item>
                <Descriptions.Item label="操作者">{event.actor_user_id}</Descriptions.Item>
                <Descriptions.Item label="时间">{new Date(event.created_at).toLocaleString()}</Descriptions.Item>
                <Descriptions.Item label="详情"><Typography.Text code>{event.detail}</Typography.Text></Descriptions.Item>
              </Descriptions>
            ),
          }))}
        />
      </Drawer>
    </PageContainer>
  );
};

export default Execution;
