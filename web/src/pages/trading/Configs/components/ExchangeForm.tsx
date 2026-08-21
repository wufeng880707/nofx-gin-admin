import {
  ModalForm,
  ProFormDependency,
  ProFormSegmented,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
} from '@ant-design/pro-components';
import type { ProFormInstance } from '@ant-design/pro-components';
import { Alert, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import { createExchange, getExchange, updateExchange } from '@/services/trading/catalog';

type Props = {
  visible: boolean;
  id?: string;
  onCancel: () => void;
  onSuccess: () => void;
};

type ExchangeFormValues = TradingAPI.ExchangeForm & {
  environment: 'production' | 'testnet';
};

const ExchangeForm: React.FC<Props> = ({ visible, id, onCancel, onSuccess }) => {
  const formRef = useRef<ProFormInstance<ExchangeFormValues>>(null);
  const [savedCredentials, setSavedCredentials] = useState({
    apiKey: false,
    secretKey: false,
    passphrase: false,
  });

  useEffect(() => {
    if (!visible) return;
    formRef.current?.resetFields();
    setSavedCredentials({ apiKey: false, secretKey: false, passphrase: false });
    if (id) {
      void getExchange(id).then((res) => {
        if (!res.data) return;
        setSavedCredentials({
          apiKey: res.data.has_api_key,
          secretKey: res.data.has_secret_key,
          passphrase: res.data.has_passphrase,
        });
        formRef.current?.setFieldsValue({
          ...res.data,
          environment: res.data.testnet ? 'testnet' : 'production',
        });
      });
    }
  }, [id, visible]);

  return (
    <ModalForm<ExchangeFormValues>
      visible={visible}
      title={id ? '编辑交易所账户' : '新增交易所账户'}
      width={720}
      formRef={formRef}
      grid
      layout="horizontal"
      initialValues={{ account_type: 'spot', enabled: false, environment: 'production' }}
      modalProps={{ destroyOnClose: true, maskClosable: false, onCancel }}
      onFinish={async (values) => {
        const { environment, ...formValues } = values;
        const payload = {
          ...formValues,
          testnet: environment === 'testnet',
          unified_account: values.account_type === 'unified',
        };
        if (id) await updateExchange(id, payload);
        else await createExchange(payload);
        message.success('交易所账户已保存');
        onSuccess();
        return true;
      }}
    >
      <Alert
        showIcon
        type="info"
        message={
          id
            ? '为避免密钥泄露，服务端不会返回凭证明文；显示“已保存”时留空即可保留原凭证。'
            : '密钥只会发送到服务端加密保存，不会在后续编辑时回显明文。'
        }
        style={{ marginBottom: 20 }}
      />
      <ProFormSelect
        name="exchange_type"
        label="交易所"
        options={[
          { label: 'Binance', value: 'binance' },
          { label: 'Bybit', value: 'bybit' },
          { label: 'OKX', value: 'okx' },
        ]}
        rules={[{ required: true }]}
        colProps={{ span: 12 }}
      />
      <ProFormText
        name="name"
        label="显示名称"
        rules={[{ required: true }]}
        colProps={{ span: 12 }}
      />
      <ProFormText
        name="account_name"
        label="账户名称"
        rules={[{ required: true }]}
        colProps={{ span: 12 }}
      />
      <ProFormDependency name={['exchange_type']}>
        {({ exchange_type }) => (
          <ProFormSelect
            name="account_type"
            label="账户类型"
            options={
              exchange_type === 'binance'
                ? [
                    { label: '现货账户（只读资产/订单）', value: 'spot' },
                    { label: 'U 本位合约', value: 'futures' },
                    { label: '统一账户（Portfolio Margin 专用）', value: 'unified' },
                  ]
                : [
                    { label: '合约', value: 'futures' },
                    { label: '现货', value: 'spot' },
                    { label: '统一账户', value: 'unified' },
                  ]
            }
            rules={[{ required: true }]}
            colProps={{ span: 12 }}
          />
        )}
      </ProFormDependency>
      <ProFormText.Password
        name="api_key"
        label="API Key"
        placeholder={savedCredentials.apiKey ? '已保存，留空保留' : '请输入'}
        fieldProps={{ autoComplete: 'new-password' }}
        colProps={{ span: 12 }}
      />
      <ProFormText.Password
        name="secret_key"
        label="Secret Key"
        placeholder={savedCredentials.secretKey ? '已保存，留空保留' : '请输入'}
        fieldProps={{ autoComplete: 'new-password' }}
        colProps={{ span: 12 }}
      />
      <ProFormDependency name={['exchange_type']}>
        {({ exchange_type }) =>
          exchange_type === 'okx' ? (
            <ProFormText.Password
              name="passphrase"
              label="Passphrase"
              placeholder={savedCredentials.passphrase ? '已保存，留空保留' : '请输入'}
              fieldProps={{ autoComplete: 'new-password' }}
              colProps={{ span: 12 }}
            />
          ) : null
        }
      </ProFormDependency>
      <ProFormSegmented
        name="environment"
        label="运行环境"
        fieldProps={{
          options: [
            { label: '正式网', value: 'production' },
            { label: '测试网', value: 'testnet' },
          ],
        }}
        colProps={{ span: 12 }}
      />
      <ProFormSwitch name="enabled" label="启用" colProps={{ span: 8 }} />
      <ProFormDependency name={['exchange_type', 'account_type', 'environment']}>
        {({ exchange_type, account_type, environment }) =>
          exchange_type === 'binance' && account_type === 'spot' ? (
            <Alert
              showIcon
              type="info"
              message="只读普通现货资产、订单和成交请选此项。系统调用 api.binance.com/api/v3/*，API Key 只需“允许读取”，无需开启现货交易或合约权限。"
              style={{ marginBottom: 20 }}
            />
          ) : exchange_type === 'binance' && account_type === 'unified' ? (
            <Alert
              showIcon
              type={environment === 'testnet' ? 'warning' : 'info'}
              message={
                environment === 'testnet'
                  ? 'Binance 统一账户仅支持正式网，请将运行环境切换为“正式网”。'
                  : 'Portfolio Margin 是需要单独开通的高级统一保证金账户，不是普通现货账户。此选项调用 papi.binance.com；如果只读普通现货资产/订单，请选“现货账户”。只读数据不需要开启合约交易权限。'
              }
              style={{ marginBottom: 20 }}
            />
          ) : null
        }
      </ProFormDependency>
    </ModalForm>
  );
};

export default ExchangeForm;
