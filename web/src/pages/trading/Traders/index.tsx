import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import {
  CloudSyncOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  TrophyOutlined,
} from '@ant-design/icons';
import { Alert, Button, Checkbox, Form, Input, message, Modal, Space, Tag, Typography } from 'antd';
import React, { useRef, useState } from 'react';
import { ActionIconButton, AddButton, DelIconButton, EditIconButton } from '@/components/Button';
import {
  deleteTrader,
  fetchTraders,
  startTrader,
  stopTrader,
  syncTrader,
} from '@/services/trading/catalog';
import TraderForm from './components/TraderForm';
import { renderDecimal } from '@/utils/decimal';
import { setCompetitionParticipation } from '@/services/trading/competition';

const Traders: React.FC = () => {
  const actionRef = useRef<ActionType>(null);
  const [formState, setFormState] = useState<{ visible: boolean; id?: string }>({ visible: false });
  const [startTarget, setStartTarget] = useState<TradingAPI.Trader>();
  const [startLoading, setStartLoading] = useState(false);
  const [startForm] = Form.useForm<TradingAPI.TraderStartForm>();
  const [competitionTarget, setCompetitionTarget] = useState<TradingAPI.Trader>();
  const [competitionLoading, setCompetitionLoading] = useState(false);
  const [competitionForm] = Form.useForm<{ public_alias: string; confirm_public_disclosure: boolean }>();

  const submitStart = async () => {
    if (!startTarget) return;
    const values = await startForm.validateFields();
    setStartLoading(true);
    try {
      await startTrader(startTarget.id, values);
      message.success('Trader 已启动账户同步和 AI 策略循环');
      setStartTarget(undefined);
      actionRef.current?.reload();
    } finally {
      setStartLoading(false);
    }
  };

  const submitCompetition = async () => {
    if (!competitionTarget) return;
    const values = await competitionForm.validateFields();
    setCompetitionLoading(true);
    try {
      await setCompetitionParticipation(competitionTarget.id, {
        participating: true,
        public_alias: values.public_alias,
        confirm_public_disclosure: values.confirm_public_disclosure,
      });
      message.success('已加入公开排行榜');
      setCompetitionTarget(undefined);
      actionRef.current?.reload();
    } finally {
      setCompetitionLoading(false);
    }
  };
  const columns: ProColumns<TradingAPI.Trader>[] = [
    { title: '名称', dataIndex: 'name', key: 'keyword', width: 160 },
    {
      title: '运行状态',
      dataIndex: 'runtime_state',
      key: 'runtimeState',
      width: 120,
      render: (_, record) => (
        <Tag color={record.runtime_state === 'running' ? 'success' : 'default'}>
          {record.runtime_state}
        </Tag>
      ),
    },
    { title: '目标状态', dataIndex: 'desired_state', search: false, width: 110 },
    {
      title: '同步 / 策略',
      search: false,
      width: 180,
      render: (_, record) => (
        <Space size={4}>
          <Tag>{record.sync_state || 'idle'}</Tag>
          <Tag color={record.strategy_state === 'running' ? 'success' : record.strategy_state === 'error' ? 'error' : 'default'}>
            {record.strategy_state || 'idle'}
          </Tag>
        </Space>
      ),
    },
    { title: '决策周期', dataIndex: 'cycle_number', search: false, width: 100 },
    { title: '循环耗时', dataIndex: 'last_cycle_duration_ms', search: false, width: 110, renderText: (value) => value > 0 ? `${value} ms` : '--' },
    {
      title: '连续失败', dataIndex: 'consecutive_failures', search: false, width: 100,
      render: (_, record) => record.consecutive_failures > 0 ? <Tag color="error">{record.consecutive_failures}</Tag> : <Tag color="success">0</Tag>,
    },
    {
      title: '安全模式',
      dataIndex: 'safe_mode',
      search: false,
      width: 100,
      render: (_, record) => <Tag color={record.safe_mode ? 'warning' : 'error'}>{record.safe_mode ? '干跑' : '交易'}</Tag>,
    },
    {
      title: '模式',
      dataIndex: 'is_simulation',
      search: false,
      width: 100,
      render: (_, record) => (
        <Tag color={record.is_simulation ? 'warning' : 'blue'}>
          {record.is_simulation ? '模拟' : '实盘'}
        </Tag>
      ),
    },
    { title: '初始资金', dataIndex: 'initial_balance', search: false, width: 130, renderText: renderDecimal },
    {
      title: '竞赛',
      dataIndex: 'show_in_competition',
      search: false,
      width: 120,
      render: (_, record) => {
        if (record.show_in_competition && record.competition_public_id) {
          return <Tag color="success">已参赛</Tag>;
        }
        if (record.show_in_competition) {
          return <Tag color="warning">待重新确认</Tag>;
        }
        return <Tag>未参加</Tag>;
      },
    },
    {
      title: '扫描间隔',
      dataIndex: 'scan_interval_minutes',
      search: false,
      width: 110,
      renderText: (value) => `${value} 分钟`,
    },
    { title: '交易所 ID', dataIndex: 'exchange_id', search: false, ellipsis: true, width: 190 },
    { title: '模型 ID', dataIndex: 'ai_model_id', search: false, ellipsis: true, width: 190 },
    {
      title: '最后心跳',
      dataIndex: 'last_heartbeat_at',
      valueType: 'dateTime',
      search: false,
      width: 180,
    },
    {
      title: '最近错误',
      dataIndex: 'last_error_message',
      search: false,
      ellipsis: true,
      width: 260,
      render: (_, record) =>
        record.last_error_message ? (
          <Typography.Text type="danger">{record.last_error_message}</Typography.Text>
        ) : (
          '-'
        ),
    },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 220,
      render: (_, record) => {
        const running =
          record.desired_state === 'running' ||
          ['starting', 'running'].includes(record.runtime_state);
        return (
          <Space size={2}>
            <EditIconButton
              code="edit"
              disabled={running}
              onClick={() => setFormState({ visible: true, id: record.id })}
            />
            {!running ? (
              <ActionIconButton
                code="start"
                tooltip="启动"
                icon={<PlayCircleOutlined />}
                onClick={() => {
                  startForm.resetFields();
                  startForm.setFieldsValue({ confirm_external_ai: false, confirm_automated_execution: false });
                  setStartTarget(record);
                }}
              />
            ) : (
              <ActionIconButton
                code="stop"
                tooltip="停止"
                confirmTitle="确认停止该 Trader？"
                danger
                icon={<PauseCircleOutlined />}
                onClick={async () => {
                  await stopTrader(record.id);
                  message.success('Trader 已停止');
                  actionRef.current?.reload();
                }}
              />
            )}
            {!running && (
              <ActionIconButton
                code="sync"
                tooltip="立即同步"
                icon={<CloudSyncOutlined />}
                onClick={async () => {
                  await syncTrader(record.id);
                  message.success('账户和仓位同步完成');
                  actionRef.current?.reload();
                }}
              />
            )}
            <ActionIconButton
              code="competition"
              tooltip={record.show_in_competition && record.competition_public_id ? '退出竞赛' : '加入竞赛'}
              confirmTitle={record.show_in_competition && record.competition_public_id ? '确认退出公开排行榜？退出后公开数据立即不可查询。' : undefined}
              danger={record.show_in_competition && Boolean(record.competition_public_id)}
              icon={<TrophyOutlined />}
              onClick={async () => {
                if (record.show_in_competition && record.competition_public_id) {
                  await setCompetitionParticipation(record.id, {
                    participating: false,
                    confirm_public_disclosure: false,
                  });
                  message.success('已退出公开排行榜');
                  actionRef.current?.reload();
                  return;
                }
                competitionForm.resetFields();
                competitionForm.setFieldsValue({
                  public_alias: record.competition_alias,
                  confirm_public_disclosure: false,
                });
                setCompetitionTarget(record);
              }}
            />
            <DelIconButton
              code="delete"
              title="确认删除该 Trader？"
              buttonProps={{ disabled: running }}
              onConfirm={async () => {
                await deleteTrader(record.id);
                message.success('Trader 已删除');
                actionRef.current?.reload();
              }}
            />
          </Space>
        );
      },
    },
  ];
  return (
    <PageContainer>
      <ProTable
        columns={columns}
        request={fetchTraders}
        actionRef={actionRef}
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 2270 }}
        options={{ density: true, reload: true, fullScreen: true }}
        toolBarRender={() => [
          <Button key="leaderboard" icon={<TrophyOutlined />} onClick={() => window.open('/competition', '_blank', 'noopener,noreferrer')}>
            公开排行榜
          </Button>,
          <AddButton key="add" code="add" onClick={() => setFormState({ visible: true })} />,
        ]}
      />
      <TraderForm
        visible={formState.visible}
        id={formState.id}
        onCancel={() => setFormState({ visible: false })}
        onSuccess={() => {
          setFormState({ visible: false });
          actionRef.current?.reload();
        }}
      />
      <Modal
        title="启动 Trader"
        open={Boolean(startTarget)}
        confirmLoading={startLoading}
        okText="确认启动"
        onOk={() => void submitStart()}
        onCancel={() => setStartTarget(undefined)}
        destroyOnClose
      >
        <Alert
          showIcon
          type={startTarget?.execution_mode === 'auto' && !startTarget?.is_simulation ? 'warning' : 'info'}
          message="启动前确认"
          description="运行期间会把策略、账户资产、仓位、公共订单簿、原始 K 线、资金费率、持仓量和已启用技术指标发送给当前选择的外部 AI Provider。服务重启后授权会自动清除。"
          style={{ marginBottom: 16 }}
        />
        <Form form={startForm} layout="vertical">
          <Form.Item
            name="confirm_external_ai"
            valuePropName="checked"
            rules={[{ validator: (_, value) => value ? Promise.resolve() : Promise.reject(new Error('必须确认外部 AI 数据传输')) }]}
          >
            <Checkbox>我确认将上述账户、仓位及扩展行情上下文发送给外部 AI Provider</Checkbox>
          </Form.Item>
          {startTarget?.execution_mode === 'auto' && !startTarget?.is_simulation && (
            <>
              <Form.Item
                name="confirm_automated_execution"
                valuePropName="checked"
                rules={[{ validator: (_, value) => value ? Promise.resolve() : Promise.reject(new Error('必须确认自动交易风险')) }]}
              >
                <Checkbox>我确认该 Trader 可以自动生成交易命令</Checkbox>
              </Form.Item>
              <Form.Item
                name="automation_phrase"
                label="自动交易确认短语"
                extra="请输入 START AUTOMATED TRADING"
                rules={[{ required: true }, { validator: (_, value) => value === 'START AUTOMATED TRADING' ? Promise.resolve() : Promise.reject(new Error('确认短语不匹配')) }]}
              >
                <Input autoComplete="off" />
              </Form.Item>
            </>
          )}
        </Form>
      </Modal>
      <Modal
        title="加入公开排行榜"
        open={Boolean(competitionTarget)}
        confirmLoading={competitionLoading}
        okText="确认公开并参赛"
        onOk={() => void submitCompetition()}
        onCancel={() => setCompetitionTarget(undefined)}
        destroyOnClose
      >
        <Alert
          showIcon
          type="warning"
          message="公开范围确认"
          description="排行榜仅公开随机公共 ID、别名、模拟/实盘标识、归一化收益率、回撤、胜率和交易数；不会公开用户 ID、内部 Trader ID、账户、绝对资产金额或凭证。"
          style={{ marginBottom: 16 }}
        />
        <Form form={competitionForm} layout="vertical">
          <Form.Item
            name="public_alias"
            label="公开别名"
            extra="请勿填写邮箱、手机号或其他个人信息；留空将由服务端生成匿名别名。"
            rules={[{ min: 2, max: 32 }]}
          >
            <Input allowClear maxLength={32} autoComplete="off" placeholder="例如 Alpha Quant" />
          </Form.Item>
          <Form.Item
            name="confirm_public_disclosure"
            valuePropName="checked"
            rules={[{ validator: (_, value) => value ? Promise.resolve() : Promise.reject(new Error('必须确认公开范围')) }]}
          >
            <Checkbox>我已了解公开字段，并主动同意该 Trader 参加排行榜</Checkbox>
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
};

export default Traders;
