import {
  ClockCircleOutlined,
  HistoryOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  PoweroffOutlined,
} from "@ant-design/icons";
import { PageContainer, ProTable } from "@ant-design/pro-components";
import type { ActionType, ProColumns } from "@ant-design/pro-components";
import {
  Alert,
  Col,
  Drawer,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Row,
  Select,
  Space,
  Tabs,
  Tag,
  Typography,
} from "antd";
import React, { useRef, useState } from "react";
import {
  ActionIconButton,
  AddButton,
  DelIconButton,
  EditIconButton,
} from "@/components/Button";
import {
  createScheduledTask,
  deleteScheduledTask,
  disableScheduledTask,
  fetchScheduledTaskAudits,
  fetchScheduledTaskRuns,
  fetchScheduledTasks,
  pauseScheduledTask,
  resumeScheduledTask,
  triggerScheduledTask,
  updateScheduledTask,
} from "@/services/trading/catalog";

const statusMeta = {
  active: { color: "success", label: "运行中" },
  paused: { color: "warning", label: "已暂停" },
  disabled: { color: "default", label: "已停用" },
} as const;

const runStatusMeta: Record<string, { color: string; label: string }> = {
  pending: { color: "processing", label: "等待中" },
  running: { color: "processing", label: "执行中" },
  success: { color: "success", label: "成功" },
  failed: { color: "error", label: "失败" },
  skipped: { color: "default", label: "已跳过" },
};

const auditActionMeta: Record<string, { color: string; label: string }> = {
  create: { color: "success", label: "创建" },
  update: { color: "processing", label: "更新" },
  delete: { color: "error", label: "删除" },
  trigger: { color: "purple", label: "手动触发" },
  pause: { color: "warning", label: "暂停" },
  resume: { color: "success", label: "恢复" },
  disable: { color: "default", label: "停用" },
};

const defaultForm: TradingAPI.ScheduledTaskForm = {
  name: "",
  handler: "account_sync",
  cron_expression: "*/5 * * * *",
  time_zone: "Asia/Shanghai",
  timeout_seconds: 60,
  max_retries: 2,
  retry_interval_seconds: 5,
  misfire_policy: "fire_once",
  concurrency_policy: "forbid",
};

const ScheduledTaskPage: React.FC = () => {
  const actionRef = useRef<ActionType>(null);
  const runActionRef = useRef<ActionType>(null);
  const auditActionRef = useRef<ActionType>(null);
  const [form] = Form.useForm<TradingAPI.ScheduledTaskForm>();
  const [editing, setEditing] = useState<TradingAPI.ScheduledTask>();
  const [formOpen, setFormOpen] = useState(false);
  const [historyTask, setHistoryTask] = useState<TradingAPI.ScheduledTask>();
  const [submitting, setSubmitting] = useState(false);
  const [actingID, setActingID] = useState<string>();

  const reload = () => actionRef.current?.reload();
  const runAction = async (
    record: TradingAPI.ScheduledTask,
    label: string,
    action: () => Promise<unknown>,
  ) => {
    setActingID(record.id);
    try {
      await action();
      message.success(`${label}成功`);
      reload();
    } finally {
      setActingID(undefined);
    }
  };

  const taskColumns: ProColumns<TradingAPI.ScheduledTask>[] = [
    { title: "任务名称", dataIndex: "name", width: 170 },
    {
      title: "处理器",
      dataIndex: "handler",
      width: 150,
      valueEnum: { account_sync: "账户数据同步", account_snapshot: "账户快照" },
      render: (_, record) =>
        record.handler === "account_sync" ? "账户数据同步" : "账户快照",
    },
    {
      title: "调度规则",
      width: 220,
      search: false,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text code>{record.cron_expression}</Typography.Text>
          <Typography.Text type="secondary">{record.time_zone}</Typography.Text>
        </Space>
      ),
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 100,
      valueType: "select",
      valueEnum: { active: "运行中", paused: "已暂停", disabled: "已停用" },
      render: (_, record) => {
        const meta = statusMeta[record.status];
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    {
      title: "执行策略",
      width: 180,
      search: false,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <span>
            超时 {record.timeout_seconds}s · 重试 {record.max_retries} 次
          </span>
          <Typography.Text type="secondary">
            {record.concurrency_policy === "forbid" ? "禁止并发" : "允许并发"} ·{" "}
            {record.misfire_policy === "fire_once" ? "错过补跑" : "错过跳过"}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: "下次执行",
      dataIndex: "next_run_at",
      valueType: "dateTime",
      width: 180,
      search: false,
      renderText: (value) => value || "--",
    },
    {
      title: "上次执行",
      dataIndex: "last_run_at",
      valueType: "dateTime",
      width: 180,
      search: false,
      renderText: (value) => value || "--",
    },
    {
      title: "操作",
      valueType: "option",
      fixed: "right",
      width: 250,
      render: (_, record) => (
        <Space size={2}>
          <EditIconButton
            code="edit"
            disabled={Boolean(actingID)}
            onClick={() => {
              setEditing(record);
              form.setFieldsValue(record);
              setFormOpen(true);
            }}
          />
          <ActionIconButton
            code="trigger"
            tooltip="立即执行"
            icon={<PlayCircleOutlined />}
            loading={actingID === record.id}
            onClick={() =>
              void runAction(record, "触发", () =>
                triggerScheduledTask(record.id),
              )
            }
          />
          {record.status === "active" ? (
            <ActionIconButton
              code="pause"
              tooltip="暂停"
              icon={<PauseCircleOutlined />}
              disabled={Boolean(actingID)}
              onClick={() =>
                void runAction(record, "暂停", () =>
                  pauseScheduledTask(record.id),
                )
              }
            />
          ) : record.status === "paused" ? (
            <ActionIconButton
              code="resume"
              tooltip="恢复"
              icon={<ClockCircleOutlined />}
              disabled={Boolean(actingID)}
              onClick={() =>
                void runAction(record, "恢复", () =>
                  resumeScheduledTask(record.id),
                )
              }
            />
          ) : null}
          {record.status !== "disabled" && (
            <ActionIconButton
              code="disable"
              tooltip="停用"
              confirmTitle="停用后需要编辑或恢复才能再次执行，确认停用？"
              icon={<PoweroffOutlined />}
              disabled={Boolean(actingID)}
              onClick={() =>
                void runAction(record, "停用", () =>
                  disableScheduledTask(record.id),
                )
              }
            />
          )}
          <ActionIconButton
            code="history"
            tooltip="执行历史"
            icon={<HistoryOutlined />}
            onClick={() => {
              setHistoryTask(record);
              window.setTimeout(() => runActionRef.current?.reload(), 0);
            }}
          />
          <DelIconButton
            code="delete"
            title="确认删除该定时任务？执行历史会保留。"
            buttonProps={{
              disabled: record.status === "active" || Boolean(actingID),
            }}
            onConfirm={async () => {
              await deleteScheduledTask(record.id);
              message.success("删除成功");
              reload();
            }}
          />
        </Space>
      ),
    },
  ];

  const runColumns: ProColumns<TradingAPI.ScheduledTaskRun>[] = [
    {
      title: "计划时间",
      dataIndex: "scheduled_at",
      valueType: "dateTime",
      width: 180,
    },
    {
      title: "来源",
      dataIndex: "trigger_source",
      width: 90,
      render: (_, row) => (row.trigger_source === "manual" ? "手动" : "定时"),
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 90,
      render: (_, row) => {
        const meta = runStatusMeta[row.status] ?? runStatusMeta.pending;
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    { title: "尝试次数", dataIndex: "attempts", width: 90 },
    {
      title: "耗时",
      dataIndex: "duration_ms",
      width: 100,
      renderText: (value) => `${value ?? 0} ms`,
    },
    {
      title: "错误信息",
      dataIndex: "error_message",
      ellipsis: true,
      renderText: (value) => value || "--",
    },
  ];

  const auditColumns: ProColumns<TradingAPI.ScheduledTaskAudit>[] = [
    {
      title: "操作时间",
      dataIndex: "created_at",
      valueType: "dateTime",
      width: 180,
    },
    {
      title: "操作者",
      dataIndex: "actor_user_id",
      width: 150,
    },
    {
      title: "动作",
      dataIndex: "action",
      width: 110,
      render: (_, row) => {
        const meta = auditActionMeta[row.action] ?? {
          color: "default",
          label: row.action,
        };
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    {
      title: "变更前",
      dataIndex: "before_json",
      ellipsis: true,
      render: (_, row) => (
        <Typography.Text
          ellipsis={{ tooltip: row.before_json || "--" }}
          style={{ maxWidth: 280 }}
        >
          {row.before_json || "--"}
        </Typography.Text>
      ),
    },
    {
      title: "变更后",
      dataIndex: "after_json",
      ellipsis: true,
      render: (_, row) => (
        <Typography.Text
          ellipsis={{ tooltip: row.after_json || "--" }}
          style={{ maxWidth: 280 }}
        >
          {row.after_json || "--"}
        </Typography.Text>
      ),
    },
  ];

  return (
    <PageContainer>
      <Alert
        showIcon
        type="info"
        message="任务定义持久化在 MySQL，仅允许系统白名单处理器；多实例通过数据库租约避免重复执行。"
        style={{ marginBottom: 16 }}
      />
      <ProTable<TradingAPI.ScheduledTask>
        actionRef={actionRef}
        rowKey="id"
        columns={taskColumns}
        request={fetchScheduledTasks}
        options={{ density: true, reload: true, fullScreen: true }}
        scroll={{ x: 1450 }}
        toolBarRender={() => [
          <AddButton
            key="add"
            code="add"
            onClick={() => {
              setEditing(undefined);
              form.setFieldsValue(defaultForm);
              setFormOpen(true);
            }}
          />,
        ]}
      />
      <Modal
        title={editing ? "编辑定时任务" : "新增定时任务"}
        open={formOpen}
        confirmLoading={submitting}
        width={760}
        onCancel={() => setFormOpen(false)}
        onOk={() => form.submit()}
        destroyOnClose
      >
        <Form
          form={form}
          layout="vertical"
          preserve={false}
          initialValues={defaultForm}
          onFinish={async (values) => {
            setSubmitting(true);
            try {
              if (editing) await updateScheduledTask(editing.id, values);
              else await createScheduledTask(values);
              message.success(editing ? "更新成功" : "创建成功");
              setFormOpen(false);
              reload();
            } finally {
              setSubmitting(false);
            }
          }}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="name"
                label="任务名称"
                rules={[{ required: true }]}
              >
                <Input maxLength={128} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="handler"
                label="处理器"
                rules={[{ required: true }]}
              >
                <Select
                  options={[
                    { value: "account_sync", label: "账户数据同步" },
                    { value: "account_snapshot", label: "账户快照" },
                  ]}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="cron_expression"
                label="Cron 表达式"
                tooltip="标准 5 段 Cron，也支持 @every 5m、@hourly 等描述符"
                rules={[{ required: true }]}
              >
                <Input />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="time_zone"
                label="时区"
                rules={[{ required: true }]}
              >
                <Select
                  showSearch
                  options={[
                    { value: "Asia/Shanghai" },
                    { value: "UTC" },
                    { value: "America/New_York" },
                  ]}
                />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="timeout_seconds"
                label="超时（秒）"
                rules={[{ required: true }]}
              >
                <InputNumber min={1} max={3600} style={{ width: "100%" }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="max_retries"
                label="失败重试次数"
                rules={[{ required: true }]}
              >
                <InputNumber min={0} max={5} style={{ width: "100%" }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="retry_interval_seconds"
                label="重试间隔（秒）"
                rules={[{ required: true }]}
              >
                <InputNumber min={1} max={600} style={{ width: "100%" }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="misfire_policy"
                label="错过执行策略"
                rules={[{ required: true }]}
              >
                <Select
                  options={[
                    { value: "fire_once", label: "恢复后补跑一次" },
                    { value: "skip", label: "跳过本次" },
                  ]}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="concurrency_policy"
                label="并发策略"
                rules={[{ required: true }]}
              >
                <Select
                  options={[
                    { value: "forbid", label: "禁止同任务并发" },
                    { value: "allow", label: "允许并发" },
                  ]}
                />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
      <Drawer
        title={`${historyTask?.name ?? ""} · 任务历史`}
        open={Boolean(historyTask)}
        width={900}
        onClose={() => setHistoryTask(undefined)}
        destroyOnClose
      >
        <Tabs
          items={[
            {
              key: "runs",
              label: "执行记录",
              children: (
                <ProTable<TradingAPI.ScheduledTaskRun>
                  actionRef={runActionRef}
                  rowKey="id"
                  columns={runColumns}
                  search={false}
                  request={(params) =>
                    fetchScheduledTaskRuns({
                      ...params,
                      taskID: historyTask?.id,
                    })
                  }
                  options={{ density: true, reload: true }}
                  scroll={{ x: 850 }}
                />
              ),
            },
            {
              key: "audits",
              label: "操作审计",
              children: (
                <ProTable<TradingAPI.ScheduledTaskAudit>
                  actionRef={auditActionRef}
                  rowKey="id"
                  columns={auditColumns}
                  search={false}
                  request={(params) =>
                    fetchScheduledTaskAudits({
                      ...params,
                      taskID: historyTask?.id,
                    })
                  }
                  options={{ density: true, reload: true }}
                  scroll={{ x: 980 }}
                />
              ),
            },
          ]}
        />
      </Drawer>
    </PageContainer>
  );
};

export default ScheduledTaskPage;
