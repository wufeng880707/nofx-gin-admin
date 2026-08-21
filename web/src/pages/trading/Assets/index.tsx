import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import { Alert, Space, Statistic, Tag, Typography } from 'antd';
import React, { useEffect, useState } from 'react';
import { fetchAssetBalances, fetchAssetSummary } from '@/services/trading/observation';
import { formatDecimal, renderDecimal } from '@/utils/decimal';

const Assets: React.FC = () => {
  const [summary, setSummary] = useState<TradingAPI.AssetSummary>();

  const loadSummary = () => {
	fetchAssetSummary()
	  .then((response) => setSummary(response.data))
	  .catch(() => setSummary(undefined));
  };

  useEffect(loadSummary, []);

  const columns: ProColumns<TradingAPI.AssetBalance>[] = [
    { title: '币种', dataIndex: 'asset', width: 90 },
    { title: '账户类型', dataIndex: 'account_type', key: 'accountType', width: 110 },
    { title: '总数量', dataIndex: 'total', search: false, width: 140, renderText: renderDecimal },
    { title: '可用数量', dataIndex: 'available', search: false, width: 140, renderText: renderDecimal },
    { title: '冻结数量', dataIndex: 'locked', search: false, width: 140, renderText: renderDecimal },
    { title: '借入', dataIndex: 'borrowed', search: false, width: 120, renderText: renderDecimal },
    { title: '利息', dataIndex: 'interest', search: false, width: 120, renderText: renderDecimal },
    { title: 'USDT 价格', dataIndex: 'price_usdt', search: false, width: 140, renderText: renderDecimal },
    {
      title: 'USDT 估值',
      dataIndex: 'value_usdt',
      search: false,
      width: 150,
      renderText: (value) => formatDecimal(value, { grouping: true }),
    },
    { title: '未实现盈亏', dataIndex: 'unrealized_pnl', search: false, width: 140, renderText: renderDecimal },
    { title: '交易所账户 ID', dataIndex: 'exchange_id', key: 'exchangeID', ellipsis: true, width: 190 },
    { title: 'Trader ID', dataIndex: 'trader_id', key: 'traderID', ellipsis: true, width: 190 },
    { title: '更新时间', dataIndex: 'captured_at', valueType: 'dateTime', search: false, width: 180 },
  ];

  return (
    <PageContainer>
      <Alert
        showIcon
        type="warning"
        message="当前总估值按已配置交易所账户汇总；同一凭证重复配置现货、合约或统一账户时可能产生资产重叠。"
        style={{ marginBottom: 16 }}
      />
      <Space size={32} wrap style={{ marginBottom: 16 }}>
        <Statistic
          title="已配置账户总估值"
          value={summary?.total_value_usdt || '0'}
          formatter={(value) => formatDecimal(value, { grouping: true })}
          suffix="USDT"
        />
        <Statistic title="资产记录" value={summary?.asset_count || 0} />
        <Space wrap>
		  {(summary?.exchanges ?? []).map((item) => (
            <Tag key={`${item.exchange_id}-${item.account_type}`}>
              {item.account_type} · {formatDecimal(item.value_usdt, { grouping: true })} USDT
            </Tag>
          ))}
        </Space>
      </Space>
      <Typography.Title level={4}>币种资产</Typography.Title>
      <ProTable
        columns={columns}
        request={async (params) => {
          const response = await fetchAssetBalances(params);
          loadSummary();
          return response;
        }}
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 1860 }}
        options={{ density: true, reload: true, fullScreen: true }}
      />
    </PageContainer>
  );
};

export default Assets;
