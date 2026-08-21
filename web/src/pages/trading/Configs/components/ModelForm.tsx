import { ModalForm, ProFormDigit, ProFormSelect, ProFormSwitch, ProFormText } from '@ant-design/pro-components';
import type { ProFormInstance } from '@ant-design/pro-components';
import { Alert, message } from 'antd';
import React, { useEffect, useRef } from 'react';
import { createModel, getModel, updateModel } from '@/services/trading/catalog';

type Props = { visible: boolean; id?: string; onCancel: () => void; onSuccess: () => void };

const ModelForm: React.FC<Props> = ({ visible, id, onCancel, onSuccess }) => {
  const formRef = useRef<ProFormInstance<TradingAPI.AIModelForm>>(null);
  useEffect(() => {
    if (!visible) return;
    formRef.current?.resetFields();
    if (id) void getModel(id).then((res) => res.data && formRef.current?.setFieldsValue(res.data));
  }, [id, visible]);

  return (
    <ModalForm<TradingAPI.AIModelForm>
      visible={visible}
      title={id ? '编辑 AI 模型' : '新增 AI 模型'}
      width={680}
      formRef={formRef}
      grid
      layout="horizontal"
      initialValues={{ provider: 'deepseek', enabled: false }}
      modalProps={{ destroyOnClose: true, maskClosable: false, onCancel }}
      onFinish={async (values) => {
        if (id) await updateModel(id, values);
        else await createModel(values);
        message.success('AI 模型已保存');
        onSuccess();
        return true;
      }}
    >
      <Alert
        showIcon
        type="info"
        message="API Key 不会回显；编辑时留空表示保留原 Key。"
        style={{ marginBottom: 20 }}
      />
      <ProFormText
        name="name"
        label="配置名称"
        rules={[{ required: true }]}
        colProps={{ span: 12 }}
      />
      <ProFormSelect
        name="provider"
        label="Provider"
        options={['deepseek', 'openai', 'anthropic', 'qwen', 'gemini', 'custom'].map((value) => ({
          label: value,
          value,
        }))}
        rules={[{ required: true }]}
        colProps={{ span: 12 }}
      />
      <ProFormText.Password name="api_key" label="API Key" colProps={{ span: 24 }} />
      <ProFormText name="custom_api_url" label="API 地址" colProps={{ span: 12 }} />
      <ProFormText name="custom_model_name" label="模型名称" colProps={{ span: 12 }} />
      <ProFormDigit
        name="input_price_per_m_tokens"
        label="输入单价（USD / 百万 Token）"
        min={0}
        fieldProps={{ precision: 8, stringMode: true }}
        colProps={{ span: 12 }}
      />
      <ProFormDigit
        name="output_price_per_m_tokens"
        label="输出单价（USD / 百万 Token）"
        min={0}
        fieldProps={{ precision: 8, stringMode: true }}
        colProps={{ span: 12 }}
      />
      <ProFormSwitch name="enabled" label="启用" colProps={{ span: 12 }} />
    </ModalForm>
  );
};

export default ModelForm;
