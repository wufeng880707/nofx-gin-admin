import {
  ModalForm,
  ProFormDependency,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
} from '@ant-design/pro-components';
import type { ProFormInstance } from '@ant-design/pro-components';
import { Alert, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import {
  createNotificationChannel,
  getNotificationChannel,
  updateNotificationChannel,
} from '@/services/trading/catalog';

type Props = {
  visible: boolean;
  id?: string;
  onCancel: () => void;
  onSuccess: () => void;
};

const NotificationForm: React.FC<Props> = ({ visible, id, onCancel, onSuccess }) => {
  const formRef = useRef<ProFormInstance<TradingAPI.NotificationChannelForm>>(null);
  const [savedCredentials, setSavedCredentials] = useState({
    botToken: false,
    webhookURL: false,
    signingSecret: false,
  });

  useEffect(() => {
    if (!visible) return;
    formRef.current?.resetFields();
    setSavedCredentials({ botToken: false, webhookURL: false, signingSecret: false });
    if (id) {
      void getNotificationChannel(id).then((res) => {
        if (!res.data) return;
        setSavedCredentials({
          botToken: res.data.has_bot_token,
          webhookURL: res.data.has_webhook_url,
          signingSecret: res.data.has_signing_secret,
        });
        formRef.current?.setFieldsValue(res.data);
      });
    }
  }, [id, visible]);

  return (
    <ModalForm<TradingAPI.NotificationChannelForm>
      visible={visible}
      title={id ? '编辑通知渠道' : '新增通知渠道'}
      width={720}
      formRef={formRef}
      grid
      layout="horizontal"
      initialValues={{ channel_type: 'lark', enabled: true }}
      modalProps={{ destroyOnClose: true, maskClosable: false, onCancel }}
      onFinish={async (values) => {
        if (id) await updateNotificationChannel(id, values);
        else await createNotificationChannel(values);
        message.success('通知渠道已保存');
        onSuccess();
        return true;
      }}
    >
      <Alert
        showIcon
        type="info"
        message={
          id
            ? '服务端不会返回 Token、Webhook 和签名密钥明文；显示“已保存”时留空即可保留原凭证。'
            : '所有通知凭证只发送到服务端加密保存。保存后请使用列表中的测试按钮验证真实消息投递。'
        }
        style={{ marginBottom: 20 }}
      />
      <ProFormText
        name="name"
        label="渠道名称"
        rules={[{ required: true }]}
        colProps={{ span: 12 }}
      />
      <ProFormSelect
        name="channel_type"
        label="渠道类型"
        rules={[{ required: true }]}
        options={[
          { label: '飞书群机器人', value: 'lark' },
          { label: 'Telegram Bot', value: 'telegram' },
        ]}
        colProps={{ span: 12 }}
      />
      <ProFormDependency name={['channel_type']}>
        {({ channel_type }) =>
          channel_type === 'telegram' ? (
            <>
              <ProFormText.Password
                name="bot_token"
                label="Bot Token"
                placeholder={savedCredentials.botToken ? '已保存，留空保留' : '从 BotFather 获取'}
                fieldProps={{ autoComplete: 'new-password' }}
                colProps={{ span: 12 }}
              />
              <ProFormText
                name="chat_id"
                label="Chat ID"
                placeholder="可留空，保存后通过一次性绑定码自动填写"
                tooltip="本地长轮询模式推荐使用绑定码；手动测试发送则需要已有 Chat ID。"
                colProps={{ span: 12 }}
              />
            </>
          ) : (
            <>
              <ProFormText.Password
                name="webhook_url"
                label="Webhook URL"
                placeholder={savedCredentials.webhookURL ? '已保存，留空保留' : '飞书自定义机器人 Webhook'}
                fieldProps={{ autoComplete: 'new-password' }}
                colProps={{ span: 24 }}
              />
              <ProFormText.Password
                name="signing_secret"
                label="签名密钥"
                placeholder={
                  savedCredentials.signingSecret ? '已保存，留空保留' : '推荐开启飞书签名校验后填写'
                }
                fieldProps={{ autoComplete: 'new-password' }}
                colProps={{ span: 12 }}
              />
            </>
          )
        }
      </ProFormDependency>
      <ProFormSwitch name="enabled" label="启用" colProps={{ span: 12 }} />
    </ModalForm>
  );
};

export default NotificationForm;
