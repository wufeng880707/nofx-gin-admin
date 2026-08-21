import { AuditOutlined, CheckOutlined, CloseOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Descriptions, Drawer, Form, Input, Modal, Space, Tag, Timeline, Typography, message } from 'antd';
import React, { useRef, useState } from 'react';
import {
  approveTradeApproval,
  fetchTradeApprovalEvents,
  fetchTradeApprovals,
  rejectTradeApproval,
} from '@/services/trading/approval';
import { formatDecimal } from '@/utils/decimal';

const statusMeta: Record<string, { color: string; label: string }> = {
  pending: { color: 'processing', label: '待审批' },
  approved: { color: 'cyan', label: '已批准' },
  executing: { color: 'warning', label: '执行中' },
  succeeded: { color: 'success', label: '已执行' },
  failed: { color: 'error', label: '执行失败' },
  rejected: { color: 'default', label: '已拒绝' },
  expired: { color: 'default', label: '已过期' },
};

const Approvals: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [active, setActive] = useState<TradingAPI.TradeApproval>();
  const [action, setAction] = useState<'approve' | 'reject'>('approve');
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [events, setEvents] = useState<TradingAPI.TradeApprovalEvent[]>([]);
  const [eventOpen, setEventOpen] = useState(false);
  const [eventLoading, setEventLoading] = useState(false);
  const [form] = Form.useForm<TradingAPI.TradeApprovalActForm>();

  const openAction = (record: TradingAPI.TradeApproval, nextAction: 'approve' | 'reject') => {
    setActive(record);
    setAction(nextAction);
    form.resetFields();
    setModalOpen(true);
  };

  const submitAction = async () => {
    if (!active) return;
    const values = await form.validateFields();
    setSubmitting(true);
    try {
      if (action === 'approve') {
        await approveTradeApproval(active.id, values);
        message.success('审批已通过，交易命令已进入现有风险校验链路');
      } else {
        await rejectTradeApproval(active.id, values);
        message.success('审批已拒绝');
      }
      setModalOpen(false);
      actionRef.current?.reload();
    } finally {
      setSubmitting(false);
    }
  };

  const showEvents = async (id: string) => {
    setEventOpen(true);
    setEventLoading(true);
    try {
      const response = await fetchTradeApprovalEvents(id);
      setEvents(response.data ?? []);
    } finally {
      setEventLoading(false);
    }
  };

  const columns: ProColumns<TradingAPI.TradeApproval>[] = [
    { title: 'Trader ID', dataIndex: 'trader_id', key: 'traderID', ellipsis: true, width: 170 },
    { title: '交易对', dataIndex: 'symbol', key: 'symbol', width: 120 },
    {
      title: '动作', dataIndex: 'action', search: false, width: 90,
      render: (_, record) => <Tag>{record.action.toUpperCase()}</Tag>,
    },
    {
      title: '方向', dataIndex: 'position_side', search: false, width: 90,
      render: (_, record) => <Tag color={record.position_side === 'long' ? 'success' : 'error'}>{record.position_side.toUpperCase()}</Tag>,
    },
    {
      title: '数量 / 名义金额', search: false, align: 'right', width: 160,
      render: (_, record) => Number(record.quantity) > 0
        ? formatDecimal(record.quantity, { maximumFractionDigits: 12 })
        : `${formatDecimal(record.quote_quantity)} USDT`,
    },
    { title: '杠杆', dataIndex: 'leverage', search: false, align: 'right', width: 80, renderText: (value) => `${value}x` },
    {
      title: '止损 / 止盈', search: false, align: 'right', width: 170,
      render: (_, record) => record.stop_loss || record.take_profit ? (
        <Space direction="vertical" size={0}>
          <Typography.Text type="danger">SL {formatDecimal(record.stop_loss, { maximumFractionDigits: 12 })}</Typography.Text>
          <Typography.Text type="success">TP {formatDecimal(record.take_profit, { maximumFractionDigits: 12 })}</Typography.Text>
        </Space>
      ) : '--',
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', valueType: 'select', width: 110,
      valueEnum: Object.fromEntries(Object.entries(statusMeta).map(([key, value]) => [key, value.label])),
      render: (_, record) => {
        const meta = statusMeta[record.status] ?? { color: 'default', label: record.status };
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    { title: '原因', dataIndex: 'reason', search: false, ellipsis: true, width: 220 },
    { title: '过期时间', dataIndex: 'expires_at', valueType: 'dateTime', search: false, width: 180 },
    {
      title: '操作', valueType: 'option', fixed: 'right', width: 190,
      render: (_, record) => [
        (record.status === 'pending' || record.status === 'approved') && <Button key="approve" type="link" size="small" icon={<CheckOutlined />} onClick={() => openAction(record, 'approve')}>{record.status === 'approved' ? '继续执行' : '批准'}</Button>,
        record.status === 'pending' && <Button key="reject" type="link" danger size="small" icon={<CloseOutlined />} onClick={() => openAction(record, 'reject')}>拒绝</Button>,
        <Button key="audit" type="link" size="small" icon={<AuditOutlined />} onClick={() => void showEvents(record.id)}>审计</Button>,
      ],
    },
  ];

  return (
    <PageContainer title="交易审批" subTitle="人工复核策略动作并进入统一交易命令链路">
      <Alert
        showIcon
        icon={<SafetyCertificateOutlined />}
        type="info"
        message="审批操作不可撤销"
        description="批准后仍会执行账户范围、交易所规则、余额、仓位和实盘开关校验。自动向 Telegram/飞书发送审批内容当前保持禁用。"
        style={{ marginBottom: 16 }}
      />
      <ProTable
        actionRef={actionRef}
        rowKey="id"
        columns={columns}
        request={fetchTradeApprovals}
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 1620 }}
        options={{ density: true, reload: true, fullScreen: true }}
      />
      <Modal
        title={action === 'approve' ? '批准交易' : '拒绝交易'}
        open={modalOpen}
        confirmLoading={submitting}
        okText={action === 'approve' ? '确认批准' : '确认拒绝'}
        okButtonProps={{ danger: action === 'reject' }}
        onOk={() => void submitAction()}
        onCancel={() => setModalOpen(false)}
        destroyOnClose
      >
        <Descriptions column={1} size="small" bordered style={{ marginBottom: 16 }}>
          <Descriptions.Item label="交易对">{active?.symbol}</Descriptions.Item>
          <Descriptions.Item label="动作">{active?.action.toUpperCase()} {active?.position_side.toUpperCase()}</Descriptions.Item>
          <Descriptions.Item label="止损 / 止盈">
            {active?.stop_loss || active?.take_profit ? `${formatDecimal(active?.stop_loss)} / ${formatDecimal(active?.take_profit)}` : '--'}
          </Descriptions.Item>
          <Descriptions.Item label="原因">{active?.reason || '--'}</Descriptions.Item>
        </Descriptions>
        <Form form={form} layout="vertical">
          {action === 'approve' ? (
            <Form.Item
              name="confirmation_phrase"
              label="确认短语"
              extra="请输入 APPROVE TRADE"
              rules={[{ required: true }, { validator: (_, value) => value === 'APPROVE TRADE' ? Promise.resolve() : Promise.reject(new Error('确认短语不匹配')) }]}
            >
              <Input autoComplete="off" />
            </Form.Item>
          ) : (
            <Form.Item name="comment" label="拒绝原因" rules={[{ required: true, message: '请输入拒绝原因' }]}>
              <Input.TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
          )}
        </Form>
      </Modal>
      <Drawer title={<Space><AuditOutlined />审批审计</Space>} width={560} open={eventOpen} loading={eventLoading} onClose={() => setEventOpen(false)}>
        <Timeline items={events.map((event) => ({
          color: event.to_status === 'failed' || event.to_status === 'rejected' ? 'red' : 'blue',
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="事件">{event.event_type}</Descriptions.Item>
              <Descriptions.Item label="状态">{event.from_status || '--'} → {event.to_status}</Descriptions.Item>
              <Descriptions.Item label="操作者">{event.actor_user_id}</Descriptions.Item>
              <Descriptions.Item label="时间">{new Date(event.created_at).toLocaleString()}</Descriptions.Item>
              <Descriptions.Item label="详情"><Typography.Text code>{event.detail}</Typography.Text></Descriptions.Item>
            </Descriptions>
          ),
        }))} />
      </Drawer>
    </PageContainer>
  );
};

export default Approvals;
