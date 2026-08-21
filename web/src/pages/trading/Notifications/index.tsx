import { ApiOutlined, DisconnectOutlined, LinkOutlined } from '@ant-design/icons';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { message, Modal, Space, Tag, Typography } from 'antd';
import React, { useRef, useState } from 'react';
import { AddButton, DelIconButton, EditIconButton } from '@/components/Button';
import {
  deleteNotificationChannel,
  createTelegramBindingCode,
  fetchNotificationChannels,
  testNotificationChannel,
  unbindTelegramSession,
} from '@/services/trading/catalog';
import NotificationForm from './components/NotificationForm';

const Notifications: React.FC = () => {
  const actionRef = useRef<ActionType>(null);
  const [editingID, setEditingID] = useState<string>();
  const [formVisible, setFormVisible] = useState(false);
  const [testingID, setTestingID] = useState<string>();

  const columns: ProColumns<TradingAPI.NotificationChannel>[] = [
    { title: '名称', dataIndex: 'name', key: 'keyword', width: 180 },
    {
      title: '类型',
      dataIndex: 'channel_type',
      key: 'channelType',
      valueType: 'select',
      valueEnum: { telegram: 'Telegram', lark: '飞书' },
      width: 120,
      render: (_, record) => (
        <Tag color={record.channel_type === 'telegram' ? 'blue' : 'cyan'}>
          {record.channel_type === 'telegram' ? 'Telegram' : '飞书'}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      valueType: 'select',
      valueEnum: { true: '启用', false: '停用' },
      width: 90,
      render: (_, record) => (
        <Tag color={record.enabled ? 'success' : 'default'}>{record.enabled ? '启用' : '停用'}</Tag>
      ),
    },
    {
      title: '接收目标',
      dataIndex: 'chat_id',
      search: false,
      ellipsis: true,
      width: 180,
      render: (_, record) => (record.channel_type === 'telegram' ? record.chat_id : '飞书群机器人'),
    },
    {
      title: '凭证',
      search: false,
      width: 100,
      render: (_, record) => {
        const ready =
          record.channel_type === 'telegram' ? record.has_bot_token : record.has_webhook_url;
        return <Tag color={ready ? 'success' : 'error'}>{ready ? '已保存' : '缺失'}</Tag>;
      },
    },
    {
      title: '最近测试',
      dataIndex: 'last_tested_at',
      valueType: 'dateTime',
      search: false,
      width: 180,
      render: (_, record) =>
        record.last_tested_at ? (
          <Space size={4}>
            <Tag color={record.last_test_status === 'success' ? 'success' : 'error'}>
              {record.last_test_status === 'success' ? '成功' : '失败'}
            </Tag>
            <span>{record.last_tested_at}</span>
          </Space>
        ) : (
          '未测试'
        ),
    },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 190,
      render: (_, record) => (
        <Space size={2}>
          <EditIconButton
            code="edit"
            onClick={() => {
              setEditingID(record.id);
              setFormVisible(true);
            }}
          />
          {record.channel_type === 'telegram' && (
            <EditIconButton
              code="bind"
              title={record.chat_id ? '解除 Telegram 绑定' : '生成 Telegram 绑定码'}
              icon={record.chat_id ? <DisconnectOutlined /> : <LinkOutlined />}
              onClick={async () => {
                if (record.chat_id) {
                  Modal.confirm({
                    title: '解除 Telegram 绑定',
                    content: '解绑后该 Chat 不能再查询账户信息，渠道 Token 会继续加密保留。',
                    okText: '确认解绑',
                    okButtonProps: { danger: true },
                    cancelText: '取消',
                    onOk: async () => {
                      await unbindTelegramSession(record.id);
                      message.success('Telegram 会话已解绑');
                      actionRef.current?.reload();
                    },
                  });
                  return;
                }
                const response = await createTelegramBindingCode(record.id);
                if (!response.data) return;
                Modal.info({
                  title: 'Telegram 一次性绑定码',
                  width: 560,
                  okText: '关闭',
                  content: (
                    <Space direction="vertical" size={12} style={{ width: '100%' }}>
                      <Typography.Text>
                        在此渠道对应的 Telegram Bot 中发送：
                      </Typography.Text>
                      <Typography.Text code copyable strong>
                        {`/bind ${response.data.code}`}
                      </Typography.Text>
                      <Typography.Text type="secondary">
                        有效期至 {new Date(response.data.expires_at).toLocaleString()}。绑定码只显示一次，服务端仅保存哈希。
                      </Typography.Text>
                    </Space>
                  ),
                });
              }}
            />
          )}
          <EditIconButton
            code="test"
            title={testingID === record.id ? '正在发送' : '发送测试消息'}
            icon={<ApiOutlined />}
            loading={testingID === record.id}
            disabled={Boolean(testingID)}
            onClick={async () => {
              const key = `notification-test-${record.id}`;
              setTestingID(record.id);
              message.loading({ key, content: '正在发送真实测试消息...', duration: 0 });
              try {
                const response = await testNotificationChannel(record.id);
                message.success({
                  key,
                  content: `消息投递成功（${response.data?.latency_ms ?? 0} ms）`,
                  duration: 5,
                });
                actionRef.current?.reload();
              } catch (error) {
                message.error({
                  key,
                  content: error instanceof Error ? error.message : '消息投递失败，请检查渠道凭证',
                  duration: 8,
                });
              } finally {
                setTestingID(undefined);
              }
            }}
          />
          <DelIconButton
            code="delete"
            title="确认删除该通知渠道？已有 Trader 绑定会同时解除。"
            onConfirm={async () => {
              await deleteNotificationChannel(record.id);
              message.success('通知渠道已删除');
              actionRef.current?.reload();
            }}
          />
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<TradingAPI.NotificationChannel>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        request={fetchNotificationChannels}
        options={{ density: true, reload: true, fullScreen: true }}
        scroll={{ x: 1100 }}
        toolBarRender={() => [
          <AddButton
            key="add"
            code="add"
            onClick={() => {
              setEditingID(undefined);
              setFormVisible(true);
            }}
          />,
        ]}
      />
      <NotificationForm
        visible={formVisible}
        id={editingID}
        onCancel={() => setFormVisible(false)}
        onSuccess={() => {
          setFormVisible(false);
          actionRef.current?.reload();
        }}
      />
    </PageContainer>
  );
};

export default Notifications;
