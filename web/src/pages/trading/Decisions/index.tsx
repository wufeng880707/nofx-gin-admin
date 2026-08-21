import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import { Descriptions, Tag, Typography } from 'antd';
import React from 'react';
import { fetchDecisions } from '@/services/trading/observation';

const jsonText = (value: unknown) => JSON.stringify(value ?? [], null, 2);

const Decisions: React.FC = () => {
  const columns: ProColumns<TradingAPI.Decision>[] = [
    { title: 'Trader ID', dataIndex: 'trader_id', key: 'traderID', ellipsis: true, width: 190 },
    { title: '交易所 ID', dataIndex: 'exchange_id', key: 'exchangeID', ellipsis: true, width: 190 },
    { title: '周期', dataIndex: 'cycle_number', search: false, width: 90 },
    {
      title: '结果',
      dataIndex: 'success',
      key: 'success',
      valueType: 'select',
      valueEnum: { true: '成功', false: '失败' },
      width: 90,
      render: (_, record) => (
        <Tag color={record.success ? 'success' : 'error'}>{record.success ? '成功' : '失败'}</Tag>
      ),
    },
    {
      title: 'AI 耗时',
      dataIndex: 'ai_request_duration_ms',
      search: false,
      width: 110,
      renderText: (value) => `${value} ms`,
    },
    {
      title: '错误信息',
      dataIndex: 'error_message',
      search: false,
      ellipsis: true,
      width: 260,
      render: (_, record) =>
        record.error_message ? (
          <Typography.Text type="danger">{record.error_message}</Typography.Text>
        ) : (
          '-'
        ),
    },
    {
      title: '决策时间',
      dataIndex: 'decided_at',
      valueType: 'dateTime',
      search: false,
      width: 180,
    },
  ];
  return (
    <PageContainer>
      <ProTable
        columns={columns}
        request={fetchDecisions}
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 1110 }}
        options={{ density: true, reload: true, fullScreen: true }}
        expandable={{
          expandedRowRender: (record) => (
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label="候选币种">
                <pre>{jsonText(record.candidate_coins)}</pre>
              </Descriptions.Item>
              <Descriptions.Item label="执行动作">
                <pre>{jsonText(record.actions)}</pre>
              </Descriptions.Item>
              <Descriptions.Item label="执行日志">
                <pre>{jsonText(record.execution_log)}</pre>
              </Descriptions.Item>
            </Descriptions>
          ),
        }}
      />
    </PageContainer>
  );
};

export default Decisions;
