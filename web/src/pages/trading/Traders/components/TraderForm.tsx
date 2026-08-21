import {
  ModalForm,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
} from '@ant-design/pro-components';
import type { ProFormInstance } from '@ant-design/pro-components';
import { Alert, message } from 'antd';
import React, { useEffect, useRef } from 'react';
import {
  createTrader,
  fetchExchanges,
  fetchModels,
  fetchNotificationChannels,
  fetchStrategies,
  getTrader,
  updateTrader,
} from '@/services/trading/catalog';

type Props = { visible: boolean; id?: string; onCancel: () => void; onSuccess: () => void };

const TraderForm: React.FC<Props> = ({ visible, id, onCancel, onSuccess }) => {
  const formRef = useRef<ProFormInstance<TradingAPI.TraderForm>>(null);
  useEffect(() => {
    if (!visible) return;
    formRef.current?.resetFields();
    if (id) void getTrader(id).then((res) => res.data && formRef.current?.setFieldsValue(res.data));
  }, [id, visible]);

  return (
    <ModalForm<TradingAPI.TraderForm>
      visible={visible}
      title={id ? '编辑 Trader' : '新增 Trader'}
      width={820}
      formRef={formRef}
      grid
      layout="horizontal"
      initialValues={{
        initial_balance: '1000',
        scan_interval_minutes: 15,
        execution_mode: 'notify_only',
        notification_channel_ids: [],
        is_cross_margin: true,
        is_simulation: true,
        auto_apply_strategy_updates: false,
      }}
      modalProps={{ destroyOnClose: true, maskClosable: false, onCancel }}
      onFinish={async (values) => {
        const payload = { ...values, initial_balance: String(values.initial_balance) };
        if (id) await updateTrader(id, payload);
        else await createTrader(payload);
        message.success('Trader 已保存');
        onSuccess();
        return true;
      }}
    >
      <Alert
        showIcon
        type="warning"
        message="执行模式、通知渠道和策略版本均由 Runtime 管理。模拟模式可显式开启已发布策略热更新；实盘始终固定版本，需停止后重新启动才能应用新版本。"
        style={{ marginBottom: 20 }}
      />
      <ProFormText name="name" label="名称" rules={[{ required: true }]} colProps={{ span: 12 }} />
      <ProFormText
        name="initial_balance"
        label="初始资金"
        rules={[{ required: true, pattern: /^\d+(\.\d+)?$/, message: '请输入非负数字' }]}
        colProps={{ span: 12 }}
      />
      <ProFormSelect
        name="exchange_id"
        label="交易所账户"
        rules={[{ required: true }]}
        request={async () => {
          const res = await fetchExchanges({
            current: 1,
            pageSize: 100,
            enabled: true,
          } as API.PaginationParam);
          return (res.data || []).map((item) => ({
            label: `${item.name} / ${item.account_name}`,
            value: item.id,
          }));
        }}
        colProps={{ span: 12 }}
      />
      <ProFormSelect
        name="ai_model_id"
        label="AI 模型"
        rules={[{ required: true }]}
        request={async () => {
          const res = await fetchModels({
            current: 1,
            pageSize: 100,
            enabled: true,
          } as API.PaginationParam);
          return (res.data || []).map((item) => ({ label: item.name, value: item.id }));
        }}
        colProps={{ span: 12 }}
      />
      <ProFormSelect
        name="strategy_id"
        label="交易策略"
        allowClear
        request={async () => {
          const res = await fetchStrategies({
            current: 1,
            pageSize: 100,
            active: true,
          } as API.PaginationParam);
          return (res.data || []).map((item) => ({ label: item.name, value: item.id }));
        }}
        colProps={{ span: 12 }}
      />
      <ProFormDigit
        name="scan_interval_minutes"
        label="同步间隔（分钟）"
        min={1}
        max={1440}
        rules={[{ required: true }]}
        colProps={{ span: 12 }}
      />
      <ProFormSelect
        name="execution_mode"
        label="交易执行模式"
        rules={[{ required: true }]}
        options={[
          { label: '仅通知（用户自行交易）', value: 'notify_only' },
          { label: '通知并等待批准', value: 'approval_required' },
          { label: '自动交易并通知结果', value: 'auto' },
        ]}
        colProps={{ span: 12 }}
      />
      <ProFormSelect
        name="notification_channel_ids"
        label="通知渠道"
        mode="multiple"
        allowClear
        placeholder="可同时选择飞书和 Telegram"
        request={async () => {
          const res = await fetchNotificationChannels({
            current: 1,
            pageSize: 100,
            enabled: true,
          } as API.PaginationParam);
          return (res.data || []).map((item) => ({
            label: `${item.name}（${item.channel_type === 'telegram' ? 'Telegram' : '飞书'}）`,
            value: item.id,
          }));
        }}
        colProps={{ span: 12 }}
      />
      <ProFormSwitch name="is_cross_margin" label="全仓模式" colProps={{ span: 8 }} />
      <ProFormSwitch name="is_simulation" label="模拟模式" colProps={{ span: 8 }} />
      <ProFormSwitch
        name="auto_apply_strategy_updates"
        label="自动应用已发布策略"
        tooltip="仅模拟模式生效，在下一个策略周期边界切换并记录审计事件"
        colProps={{ span: 8 }}
      />
    </ModalForm>
  );
};

export default TraderForm;
