import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ApiOutlined, HistoryOutlined, SyncOutlined } from '@ant-design/icons';
import { message, Space, Tabs, Tag } from 'antd';
import React, { useRef, useState } from 'react';
import { AddButton, DelIconButton, EditIconButton } from '@/components/Button';
import {
  deleteExchange,
  deleteModel,
  fetchExchanges,
  fetchModels,
  syncExchange,
  testExchange,
  testModel,
} from '@/services/trading/catalog';
import ExchangeForm from './components/ExchangeForm';
import HistoryBackfillForm from './components/HistoryBackfillForm';
import ModelForm from './components/ModelForm';

const enabledTag = (enabled: boolean) => (
  <Tag color={enabled ? 'success' : 'default'}>{enabled ? '启用' : '停用'}</Tag>
);

const tableOptions = { density: true, reload: true, fullScreen: true };

type RequestError = {
  message?: string;
  data?: API.ResponseResult<unknown>;
  response?: { data?: API.ResponseResult<unknown> };
};

const formatExchangeTestError = (detail: string, record?: TradingAPI.Exchange) => {
  if (detail.includes('does not support testnet') && detail.includes('Binance Portfolio Margin')) {
    return 'Binance 统一账户（Portfolio Margin）不支持测试网，请将运行环境切换为“正式网”';
  }
  if (detail.includes('code":-2015') || detail.includes('Invalid API-key, IP, or permissions')) {
    if (record?.exchange_type === 'binance' && record.account_type === 'unified') {
      return 'Binance Portfolio Margin 接口拒绝访问（-2015）：当前配置调用 papi.binance.com，只适用于已开通 Portfolio Margin 的账户。如果只想读取普通现货资产/订单，请将账户类型改为“现货账户”；不需要开启合约交易权限';
    }
    if (record?.exchange_type === 'binance' && record.account_type === 'spot') {
      return 'Binance 现货只读鉴权失败（-2015）：请检查 API Key/Secret Key 是否配对、“允许读取”是否开启，以及当前公网 IP 是否符合白名单设置；无需开启现货交易或合约权限';
    }
    return 'Binance U 本位合约鉴权失败（-2015）：请检查 API Key/Secret Key、IP 白名单以及账户是否已开通合约产品；如果当前只需现货数据，请将账户类型改为“现货账户”';
  }
  if (detail.includes('Exchange network is reachable, but account authentication failed')) {
    return detail.replace(
      /^Exchange network is reachable, but account authentication failed:\s*/i,
      '交易所接口可访问，但账户鉴权失败：',
    );
  }
  if (detail.includes('Unable to reach the configured exchange environment')) {
    return detail.replace(
      /^Unable to reach the configured exchange environment:\s*/i,
      '无法访问所选交易所环境：',
    );
  }
  return detail.replace(/^Exchange connection test failed:\s*/i, '交易所连接测试失败：');
};

const getRequestErrorDetail = (error: unknown, record?: TradingAPI.Exchange) => {
  if (!error || typeof error !== 'object') {
    return '交易所连接测试失败，请检查 API Key、IP 白名单和账户权限';
  }
  const requestError = error as RequestError;
  const detail =
    requestError.data?.error?.detail ||
    requestError.response?.data?.error?.detail ||
    requestError.message ||
    '交易所连接测试失败，请检查 API Key、IP 白名单和账户权限';
  return formatExchangeTestError(detail, record);
};

const Configs: React.FC = () => {
  const exchangeRef = useRef<ActionType>(null);
  const modelRef = useRef<ActionType>(null);
  const [modal, setModal] = useState<{ type: 'exchange' | 'model'; id?: string }>();
  const [testingExchangeID, setTestingExchangeID] = useState<string>();
  const [testingModelID, setTestingModelID] = useState<string>();
  const [syncingExchangeID, setSyncingExchangeID] = useState<string>();
  const [backfillExchange, setBackfillExchange] = useState<TradingAPI.Exchange>();

  const handleTestExchange = async (record: TradingAPI.Exchange) => {
    if (testingExchangeID) return;

    const messageKey = `exchange-test-${record.id}`;
    setTestingExchangeID(record.id);
    message.loading({ key: messageKey, content: '正在测试交易所连接...', duration: 0 });
    try {
      const result = await testExchange(record.id);
      const data = result.data;
      const environment = data?.environment === 'testnet' ? '测试网' : '正式网';
      message.success({
        key: messageKey,
        content:
          data === undefined
            ? '交易所连接正常'
            : `${environment}连接正常：网络 ${data.network_latency_ms} ms，账户鉴权 ${data.authentication_latency_ms} ms`,
        duration: 5,
      });
    } catch (error) {
      message.error({
        key: messageKey,
        content: getRequestErrorDetail(error, record),
        duration: 8,
      });
    } finally {
      setTestingExchangeID(undefined);
    }
  };

  const handleSyncExchange = async (record: TradingAPI.Exchange) => {
    if (syncingExchangeID) return;
    const messageKey = `exchange-sync-${record.id}`;
    setSyncingExchangeID(record.id);
    message.loading({ key: messageKey, content: '正在同步账户资产与历史订单...', duration: 0 });
    try {
      const response = await syncExchange(record.id);
      const data = response.data;
      message.success({
        key: messageKey,
        content: data
          ? `同步完成：资产 ${data.asset_count}，订单 ${data.order_count}，成交 ${data.fill_count}，收益事件 ${data.income_event_count}`
          : '同步完成',
        duration: 8,
      });
    } catch (error) {
      message.error({
        key: messageKey,
        content: getRequestErrorDetail(error, record),
        duration: 10,
      });
    } finally {
      setSyncingExchangeID(undefined);
    }
  };

  const handleTestModel = async (record: TradingAPI.AIModel) => {
    if (testingModelID) return;
    const messageKey = `model-test-${record.id}`;
    setTestingModelID(record.id);
    message.loading({ key: messageKey, content: '正在调用模型正式接口...', duration: 0 });
    try {
      const response = await testModel(record.id);
      const data = response.data;
      message.success({
        key: messageKey,
        content: data
          ? `${data.provider} / ${data.model_name} 可用：${data.latency_ms} ms，${data.total_tokens} tokens，响应 ${data.response}`
          : 'AI 模型连接正常',
        duration: 8,
      });
    } catch (error) {
      const requestError = error as RequestError;
      const detail = requestError.data?.error?.detail
        || requestError.response?.data?.error?.detail
        || requestError.message
        || 'AI 模型连接测试失败，请检查 API Key、模型名称和服务地址';
      message.error({ key: messageKey, content: detail, duration: 10 });
    } finally {
      setTestingModelID(undefined);
    }
  };

  const exchangeColumns: ProColumns<TradingAPI.Exchange>[] = [
    { title: '名称', dataIndex: 'name', key: 'keyword', width: 160 },
    { title: '交易所', dataIndex: 'exchange_type', key: 'exchangeType', width: 120 },
    { title: '账户名称', dataIndex: 'account_name', search: false, width: 160 },
    {
      title: '账户类型',
      dataIndex: 'account_type',
      search: false,
      width: 180,
      render: (_, record) => {
        if (record.account_type === 'spot') return '现货账户（只读）';
        if (record.account_type === 'unified') return 'Portfolio Margin（专用）';
        return record.exchange_type === 'binance' ? 'U 本位合约' : '合约';
      },
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      valueType: 'select',
      valueEnum: { true: '启用', false: '停用' },
      width: 90,
      render: (_, record) => enabledTag(record.enabled),
    },
    {
      title: '环境',
      dataIndex: 'testnet',
      search: false,
      width: 90,
      render: (_, record) => (
        <Tag color={record.testnet ? 'warning' : 'blue'}>
          {record.testnet ? '测试网' : '正式网'}
        </Tag>
      ),
    },
    {
      title: '凭证',
      search: false,
      width: 120,
      render: (_, record) => enabledTag(record.has_api_key && record.has_secret_key),
    },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      valueType: 'dateTime',
      search: false,
      width: 180,
    },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 230,
      render: (_, record) => (
        <Space size={2}>
          <EditIconButton
            code="edit"
            onClick={() => setModal({ type: 'exchange', id: record.id })}
          />
          <EditIconButton
            code="test"
            title={testingExchangeID === record.id ? '正在测试连接' : '测试连接'}
            icon={<ApiOutlined />}
            loading={testingExchangeID === record.id}
            disabled={Boolean(testingExchangeID)}
            onClick={() => handleTestExchange(record)}
          />
          <EditIconButton
            code="sync-data"
            title={syncingExchangeID === record.id ? '正在同步数据' : '同步账户数据'}
            icon={<SyncOutlined />}
            loading={syncingExchangeID === record.id}
            disabled={Boolean(syncingExchangeID)}
            onClick={() => handleSyncExchange(record)}
          />
          {record.exchange_type !== 'bybit' && (
            <EditIconButton
              code="history-backfill"
              title="历史订单回填"
              icon={<HistoryOutlined />}
              onClick={() => setBackfillExchange(record)}
            />
          )}
          <DelIconButton
            code="delete"
            title="确认删除该交易所账户？"
            onConfirm={async () => {
              await deleteExchange(record.id);
              message.success('交易所账户已删除');
              exchangeRef.current?.reload();
            }}
          />
        </Space>
      ),
    },
  ];
  const modelColumns: ProColumns<TradingAPI.AIModel>[] = [
    { title: '名称', dataIndex: 'name', key: 'keyword', width: 160 },
    { title: 'Provider', dataIndex: 'provider', key: 'provider', width: 140 },
    { title: '模型名称', dataIndex: 'custom_model_name', search: false, width: 180 },
    { title: 'API 地址', dataIndex: 'custom_api_url', search: false, ellipsis: true, width: 240 },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      valueType: 'select',
      valueEnum: { true: '启用', false: '停用' },
      width: 90,
      render: (_, record) => enabledTag(record.enabled),
    },
    {
      title: 'API Key',
      search: false,
      width: 100,
      render: (_, record) => enabledTag(record.has_api_key),
    },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      valueType: 'dateTime',
      search: false,
      width: 180,
    },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 140,
      render: (_, record) => (
        <Space size={2}>
          <EditIconButton code="edit" onClick={() => setModal({ type: 'model', id: record.id })} />
          <EditIconButton
            code="test"
            title={testingModelID === record.id ? '正在测试模型' : '测试模型正式接口'}
            icon={<ApiOutlined />}
            loading={testingModelID === record.id}
            disabled={Boolean(testingModelID) || !record.has_api_key}
            onClick={() => handleTestModel(record)}
          />
          <DelIconButton
            code="delete"
            title="确认删除该 AI 模型？"
            onConfirm={async () => {
              await deleteModel(record.id);
              message.success('AI 模型已删除');
              modelRef.current?.reload();
            }}
          />
        </Space>
      ),
    },
  ];
  return (
    <PageContainer>
      <Tabs
        items={[
          {
            key: 'exchanges',
            label: '交易所账户',
            children: (
              <ProTable
                columns={exchangeColumns}
                request={fetchExchanges}
                actionRef={exchangeRef}
                rowKey="id"
                search={{ labelWidth: 'auto' }}
                pagination={{ pageSize: 10 }}
                scroll={{ x: 1260 }}
                options={tableOptions}
                toolBarRender={() => [
                  <AddButton key="add" code="add" onClick={() => setModal({ type: 'exchange' })} />,
                ]}
              />
            ),
          },
          {
            key: 'models',
            label: 'AI 模型',
            children: (
              <ProTable
                columns={modelColumns}
                request={fetchModels}
                actionRef={modelRef}
                rowKey="id"
                search={{ labelWidth: 'auto' }}
                pagination={{ pageSize: 10 }}
                scroll={{ x: 1190 }}
                options={tableOptions}
                toolBarRender={() => [
                  <AddButton key="add" code="add" onClick={() => setModal({ type: 'model' })} />,
                ]}
              />
            ),
          },
        ]}
      />
      <ExchangeForm
        visible={modal?.type === 'exchange'}
        id={modal?.type === 'exchange' ? modal.id : undefined}
        onCancel={() => setModal(undefined)}
        onSuccess={() => {
          setModal(undefined);
          exchangeRef.current?.reload();
        }}
      />
      <HistoryBackfillForm
        visible={Boolean(backfillExchange)}
        exchange={backfillExchange}
        onCancel={() => setBackfillExchange(undefined)}
        onSuccess={() => {
          setBackfillExchange(undefined);
          exchangeRef.current?.reload();
        }}
      />
      <ModelForm
        visible={modal?.type === 'model'}
        id={modal?.type === 'model' ? modal.id : undefined}
        onCancel={() => setModal(undefined)}
        onSuccess={() => {
          setModal(undefined);
          modelRef.current?.reload();
        }}
      />
    </PageContainer>
  );
};

export default Configs;
