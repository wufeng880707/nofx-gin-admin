import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import React from 'react';
import { fetchIncomeEvents } from '@/services/trading/observation';
import { renderDecimal } from '@/utils/decimal';

const IncomeEvents: React.FC = () => {
  const columns: ProColumns<TradingAPI.IncomeEvent>[] = [
    { title: '事件类型', dataIndex: 'event_type', key: 'eventType', width: 170 },
    { title: '交易对', dataIndex: 'symbol', width: 130 },
    { title: '市场', dataIndex: 'market', width: 100 },
    { title: '资产', dataIndex: 'asset', width: 90 },
    { title: '原始数量', dataIndex: 'amount', search: false, width: 160, renderText: renderDecimal },
    { title: 'USDT 价格', dataIndex: 'price_usdt', search: false, width: 140, renderText: renderDecimal },
    { title: 'USDT 金额', dataIndex: 'value_usdt', search: false, width: 160, renderText: renderDecimal },
    { title: '交易所账户 ID', dataIndex: 'exchange_id', key: 'exchangeID', ellipsis: true, width: 200 },
    { title: 'Trader ID', dataIndex: 'trader_id', key: 'traderID', ellipsis: true, width: 200 },
    { title: '发生时间', dataIndex: 'occurred_at', valueType: 'dateTime', search: false, width: 180 },
  ];

  return (
    <PageContainer>
      <ProTable
        columns={columns}
        request={fetchIncomeEvents}
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 1530 }}
        options={{ density: true, reload: true, fullScreen: true }}
      />
    </PageContainer>
  );
};

export default IncomeEvents;
