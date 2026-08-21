import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import React from 'react';
import { fetchSnapshots } from '@/services/trading/observation';
import { renderDecimal } from '@/utils/decimal';

const Snapshots: React.FC = () => {
  const columns: ProColumns<TradingAPI.AccountSnapshot>[] = [
    { title: 'Trader ID', dataIndex: 'trader_id', key: 'traderID', ellipsis: true, width: 190 },
    { title: '交易所 ID', dataIndex: 'exchange_id', key: 'exchangeID', ellipsis: true, width: 190 },
    { title: '总权益', dataIndex: 'total_equity', search: false, width: 130, renderText: renderDecimal },
    { title: '余额', dataIndex: 'balance', search: false, width: 130, renderText: renderDecimal },
    { title: '可用余额', dataIndex: 'available_balance', search: false, width: 130, renderText: renderDecimal },
    { title: '未实现盈亏', dataIndex: 'unrealized_pnl', search: false, width: 130, renderText: renderDecimal },
    { title: '仓位数', dataIndex: 'position_count', search: false, width: 90 },
    { title: '保证金占用', dataIndex: 'margin_used_pct', search: false, width: 120, renderText: renderDecimal },
    { title: '来源', dataIndex: 'source', search: false, width: 100 },
    {
      title: '采集时间',
      dataIndex: 'captured_at',
      valueType: 'dateTime',
      search: false,
      width: 180,
    },
  ];
  return (
    <PageContainer>
      <ProTable
        columns={columns}
        request={fetchSnapshots}
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 1390 }}
        options={{ density: true, reload: true, fullScreen: true }}
      />
    </PageContainer>
  );
};

export default Snapshots;
