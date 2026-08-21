import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import { Tag } from 'antd';
import React from 'react';
import { fetchFills } from '@/services/trading/observation';
import { renderDecimal } from '@/utils/decimal';

const Fills: React.FC = () => {
  const columns: ProColumns<TradingAPI.Fill>[] = [
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
    { title: '持仓方向', dataIndex: 'position_side', search: false, width: 110 },
    { title: '成交价格', dataIndex: 'price', search: false, width: 140, renderText: renderDecimal },
    { title: '成交数量', dataIndex: 'quantity', search: false, width: 140, renderText: renderDecimal },
    { title: '成交额', dataIndex: 'quote_quantity', search: false, width: 140, renderText: renderDecimal },
    { title: '手续费币种', dataIndex: 'fee_asset', search: false, width: 110 },
    { title: '手续费数量', dataIndex: 'fee_quantity', search: false, width: 140, renderText: renderDecimal },
    { title: '手续费 USDT', dataIndex: 'fee_value_usdt', search: false, width: 140, renderText: renderDecimal },
    { title: '已实现盈亏', dataIndex: 'realized_pnl', search: false, width: 140, renderText: renderDecimal },
    { title: '流动性', dataIndex: 'liquidity', search: false, width: 100 },
    { title: '交易所成交号', dataIndex: 'exchange_trade_id', search: false, ellipsis: true, width: 180 },
    { title: '交易所订单号', dataIndex: 'exchange_order_id', search: false, ellipsis: true, width: 180 },
    { title: '交易所账户 ID', dataIndex: 'exchange_id', key: 'exchangeID', ellipsis: true, width: 190 },
    { title: '成交时间', dataIndex: 'filled_at', valueType: 'dateTime', search: false, width: 180 },
  ];

  return (
    <PageContainer>
      <ProTable
        columns={columns}
        request={fetchFills}
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 2230 }}
        options={{ density: true, reload: true, fullScreen: true }}
      />
    </PageContainer>
  );
};

export default Fills;
