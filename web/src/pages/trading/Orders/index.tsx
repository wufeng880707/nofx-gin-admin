import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Button, Modal, Tag, Typography, message } from 'antd';
import React, { useRef } from 'react';
import { fetchOrders } from '@/services/trading/observation';
import { createTradeCommand } from '@/services/trading/execution';
import { renderDecimal } from '@/utils/decimal';

const statusColor = (status: string) => {
  if (status === 'filled') return 'success';
  if (status === 'partially_filled' || status === 'live') return 'processing';
  if (status === 'rejected' || status === 'failed') return 'error';
  return 'default';
};

const Orders: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const cancelOrder = (order: TradingAPI.ExchangeOrder) => {
    Modal.confirm({
      title: `撤销 ${order.symbol} 订单`,
      content: `交易所订单号：${order.exchange_order_id}`,
      okText: '确认创建撤单命令',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        const response = await createTradeCommand({
          trader_id: order.trader_id,
          idempotency_key: `manual-cancel-${order.id}-${Date.now()}`,
          action: 'cancel',
          market: order.market as TradingAPI.MarketType,
          symbol: order.symbol,
          target_exchange_order_id: order.exchange_order_id,
          confirm_execution: true,
        });
        message.success(response.data?.simulation ? '模拟撤单命令已记录' : '撤单命令已提交');
        actionRef.current?.reload();
      },
    });
  };
  const columns: ProColumns<TradingAPI.ExchangeOrder>[] = [
    { title: '交易对', dataIndex: 'symbol', width: 130 },
    { title: '市场', dataIndex: 'market', width: 90 },
    {
      title: '方向',
      dataIndex: 'side',
      valueType: 'select',
      valueEnum: { buy: '买入', sell: '卖出' },
      width: 90,
      render: (_, record) => <Tag color={record.side === 'buy' ? 'success' : 'error'}>{record.side}</Tag>,
    },
    { title: '订单类型', dataIndex: 'order_type', search: false, width: 130 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 140,
      render: (_, record) => <Tag color={statusColor(record.status)}>{record.status}</Tag>,
    },
    {
      title: '来源',
      dataIndex: 'origin',
      valueType: 'select',
      valueEnum: { system: 'XTrading', imported: '历史导入', external: '外部订单' },
      width: 110,
    },
    { title: '委托数量', dataIndex: 'quantity', search: false, width: 130, renderText: renderDecimal },
    { title: '成交数量', dataIndex: 'executed_quantity', search: false, width: 130, renderText: renderDecimal },
    { title: '委托价格', dataIndex: 'price', search: false, width: 130, renderText: renderDecimal },
    { title: '成交均价', dataIndex: 'average_price', search: false, width: 130, renderText: renderDecimal },
    { title: '成交额', dataIndex: 'quote_quantity', search: false, width: 130, renderText: renderDecimal },
    { title: '交易所订单号', dataIndex: 'exchange_order_id', search: false, ellipsis: true, width: 180 },
    { title: '客户端订单号', dataIndex: 'client_order_id', search: false, ellipsis: true, width: 180 },
    { title: '交易所账户 ID', dataIndex: 'exchange_id', key: 'exchangeID', ellipsis: true, width: 190 },
    { title: '下单时间', dataIndex: 'ordered_at', valueType: 'dateTime', search: false, width: 180 },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 100,
      render: (_, record) =>
        ['live', 'partially_filled'].includes(record.status) && record.trader_id && record.exchange_order_id ? (
          <Button danger type="link" size="small" onClick={() => cancelOrder(record)}>
            撤单
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
        request={fetchOrders}
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 2280 }}
        options={{ density: true, reload: true, fullScreen: true }}
      />
    </PageContainer>
  );
};

export default Orders;
