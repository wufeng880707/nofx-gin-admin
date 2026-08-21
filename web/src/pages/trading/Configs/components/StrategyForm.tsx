import { ModalForm, ProFormSwitch, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import type { ProFormInstance } from '@ant-design/pro-components';
import { message } from 'antd';
import React, { useEffect, useRef } from 'react';
import { createStrategy, getStrategy, updateStrategy } from '@/services/trading/catalog';

type FormValues = Omit<TradingAPI.StrategyForm, 'config'> & { config_text: string };
type Props = { visible: boolean; id?: string; onCancel: () => void; onSuccess: () => void };

const StrategyForm: React.FC<Props> = ({ visible, id, onCancel, onSuccess }) => {
  const formRef = useRef<ProFormInstance<FormValues>>(null);
  useEffect(() => {
    if (!visible) return;
    formRef.current?.resetFields();
    if (id) {
      void getStrategy(id).then((res) => {
        if (res.data) {
          formRef.current?.setFieldsValue({
            ...res.data,
            config_text: JSON.stringify(res.data.config ?? {}, null, 2),
          });
        }
      });
    }
  }, [id, visible]);

  return (
    <ModalForm<FormValues>
      visible={visible}
      title={id ? '编辑交易策略' : '新增交易策略'}
      width={760}
      formRef={formRef}
      grid
      layout="horizontal"
      initialValues={{
        strategy_type: 'ai_trading',
        is_active: false,
        is_default: false,
        is_public: false,
        config_visible: true,
        config_text: '{}',
      }}
      modalProps={{ destroyOnClose: true, maskClosable: false, onCancel }}
      onFinish={async (values) => {
        let config: Record<string, unknown>;
        try {
          config = JSON.parse(values.config_text);
          if (!config || Array.isArray(config) || typeof config !== 'object') throw new Error();
        } catch {
          message.error('策略配置必须是 JSON 对象');
          return false;
        }
        const { config_text: _configText, ...base } = values;
        const payload = { ...base, config } as TradingAPI.StrategyForm;
        if (id) await updateStrategy(id, payload);
        else await createStrategy(payload);
        message.success('交易策略已保存');
        onSuccess();
        return true;
      }}
    >
      <ProFormText
        name="name"
        label="策略名称"
        rules={[{ required: true }]}
        colProps={{ span: 12 }}
      />
      <ProFormText
        name="strategy_type"
        label="策略类型"
        rules={[{ required: true }]}
        colProps={{ span: 12 }}
      />
      <ProFormTextArea name="description" label="描述" colProps={{ span: 24 }} />
      <ProFormSwitch name="is_active" label="启用" colProps={{ span: 6 }} />
      <ProFormSwitch name="is_default" label="默认" colProps={{ span: 6 }} />
      <ProFormSwitch name="is_public" label="公开" colProps={{ span: 6 }} />
      <ProFormSwitch name="config_visible" label="配置可见" colProps={{ span: 6 }} />
      <ProFormTextArea
        name="config_text"
        label="策略配置 JSON"
        fieldProps={{ rows: 12, style: { fontFamily: 'monospace' } }}
        rules={[{ required: true }]}
        colProps={{ span: 24 }}
      />
    </ModalForm>
  );
};

export default StrategyForm;
