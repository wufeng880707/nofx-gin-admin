import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Button, Checkbox, Input, Modal, Space, Tag, Typography, message } from 'antd';
import React, { useRef, useState } from 'react';
import { fetchPositions } from '@/services/trading/observation';
import { bulkClosePositions, createTradeCommand } from '@/services/trading/execution';
import { renderDecimal } from '@/utils/decimal';

const Positions: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [bulkOpen, setBulkOpen] = useState(false);
  const [bulkPhrase, setBulkPhrase] = useState('');
  const [bulkRisk, setBulkRisk] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const closePosition = (position: TradingAPI.Position) => {
    Modal.confirm({
      title: `平仓 ${position.symbol}`,
      content: '该操作将创建 Reduce-Only 市价平仓命令，并经过服务端风控和实盘门禁。',
      okText: '确认创建平仓命令',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        const response = await createTradeCommand({
          trader_id: position.trader_id,
          idempotency_key: `manual-close-${position.id}-${Date.now()}`,
          action: 'close',
          market: 'futures',
          symbol: position.symbol,
          side: position.side === 'short' ? 'buy' : 'sell',
          position_side: position.side as 'long' | 'short',
          order_type: 'market',
          quantity: position.quantity,
          leverage: position.leverage || 1,
          reduce_only: true,
          confirm_execution: true,
        });
        message.success(response.data?.simulation ? '模拟平仓命令已记录' : '平仓命令已提交');
        actionRef.current?.reload();
      },
    });
  };

  const submitBulkClose = async () => {
    setSubmitting(true);
    try {
      const response = await bulkClosePositions({
        operation_id: globalThis.crypto?.randomUUID?.() ?? `bulk-${Date.now()}`,
        confirmation_phrase: bulkPhrase,
        acknowledge_risk: bulkRisk,
        confirm_execution: true,
      });
      const result = response.data;
      if (result?.failed) message.warning(`已创建 ${result.succeeded} 条命令，${result.failed} 条失败`);
      else message.success(`已创建 ${result?.succeeded ?? 0} 条平仓命令`);
      setBulkOpen(false);
      setBulkPhrase('');
      setBulkRisk(false);
      actionRef.current?.reload();
    } finally {
      setSubmitting(false);
    }
  };

  const columns: ProColumns<TradingAPI.Position>[] = [
    { title: '交易对', dataIndex: 'symbol', key: 'symbol', width: 120 },
    {
      title: '方向',
      dataIndex: 'side',
      key: 'side',
      valueType: 'select',
      valueEnum: { long: 'Long', short: 'Short' },
      width: 90,
      render: (_, record) => (
        <Tag color={record.side === 'long' ? 'success' : 'error'}>{record.side.toUpperCase()}</Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      valueType: 'select',
      valueEnum: { open: 'Open', closed: 'Closed' },
      width: 100,
    },
    { title: '数量', dataIndex: 'quantity', search: false, width: 120, renderText: renderDecimal },
    { title: '开仓价', dataIndex: 'entry_price', search: false, width: 120, renderText: renderDecimal },
    { title: '标记价', dataIndex: 'mark_price', search: false, width: 120, renderText: renderDecimal },
    { title: '强平价', dataIndex: 'liquidation_price', search: false, width: 120, renderText: renderDecimal },
    { title: '未实现盈亏', dataIndex: 'unrealized_pnl', search: false, width: 130, renderText: renderDecimal },
    { title: '已实现盈亏', dataIndex: 'realized_pnl', search: false, width: 130, renderText: renderDecimal },
    {
      title: '杠杆',
      dataIndex: 'leverage',
      search: false,
      width: 80,
      renderText: (value) => `${value}x`,
    },
    { title: 'Trader ID', dataIndex: 'trader_id', key: 'traderID', ellipsis: true, width: 180 },
    { title: '开仓时间', dataIndex: 'opened_at', valueType: 'dateTime', search: false, width: 180 },
    {
      title: '同步时间',
      dataIndex: 'last_synced_at',
      valueType: 'dateTime',
      search: false,
      width: 180,
    },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 100,
      render: (_, record) =>
        record.status === 'open' && record.trader_id ? (
          <Button danger type="link" size="small" onClick={() => closePosition(record)}>
            平仓
          </Button>
        ) : (
          <Typography.Text type="secondary">--</Typography.Text>
        ),
    },
  ];
  return (
    <PageContainer>
      <ProTable
        actionRef={actionRef}
        columns={columns}
        request={fetchPositions}
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 1790 }}
        options={{ density: true, reload: true, fullScreen: true }}
        toolBarRender={() => [
          <Button danger key="close-all" onClick={() => setBulkOpen(true)}>
            全部平仓
          </Button>,
        ]}
      />
      <Modal
        open={bulkOpen}
        title="全部平仓"
        okText="创建全部平仓命令"
        okButtonProps={{ danger: true, disabled: bulkPhrase !== 'CLOSE ALL POSITIONS' || !bulkRisk }}
        confirmLoading={submitting}
        onOk={() => void submitBulkClose()}
        onCancel={() => setBulkOpen(false)}
      >
        <Space direction="vertical" size={14} style={{ width: '100%' }}>
          <Typography.Text type="warning">
            系统会对当前用户全部开放仓位逐条创建 Reduce-Only 市价平仓命令。每条命令仍需通过应用风控、用户开关和服务端总开关。
          </Typography.Text>
          <Typography.Text>请输入 <Typography.Text code>CLOSE ALL POSITIONS</Typography.Text></Typography.Text>
          <Input value={bulkPhrase} onChange={(event) => setBulkPhrase(event.target.value)} />
          <Checkbox checked={bulkRisk} onChange={(event) => setBulkRisk(event.target.checked)}>
            我已核对当前仓位并理解批量平仓风险
          </Checkbox>
        </Space>
      </Modal>
    </PageContainer>
  );
};

export default Positions;
