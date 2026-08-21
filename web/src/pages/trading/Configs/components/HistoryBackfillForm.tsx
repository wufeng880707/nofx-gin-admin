import {
  ModalForm,
  ProFormDateTimeRangePicker,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { Alert, message } from 'antd';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import React from 'react';
import { backfillExchangeHistory } from '@/services/trading/catalog';

type BackfillValues = {
  range: [Dayjs, Dayjs];
  symbols?: string;
};

type Props = {
  exchange?: Pick<TradingAPI.Exchange, 'id' | 'name' | 'exchange_type'>;
  visible: boolean;
  onCancel: () => void;
  onSuccess: () => void;
};

const parseSymbols = (value?: string) =>
  Array.from(
    new Set(
      (value || '')
        .split(/[\s,;]+/)
        .map((symbol) => symbol.trim().toUpperCase())
        .filter(Boolean),
    ),
  );

const HistoryBackfillForm: React.FC<Props> = ({ exchange, visible, onCancel, onSuccess }) => (
  <ModalForm<BackfillValues>
    visible={visible}
    title={`历史订单回填${exchange ? ` - ${exchange.name}` : ''}`}
    width={600}
    initialValues={{ range: [dayjs().subtract(7, 'day'), dayjs()] }}
    modalProps={{ destroyOnClose: true, maskClosable: false, onCancel }}
    submitter={{ searchConfig: { submitText: '开始回填', resetText: '取消' } }}
    onFinish={async (values) => {
      if (!exchange) return false;
      const [startAt, endAt] = values.range;
      const symbols = parseSymbols(values.symbols);
      if (exchange.exchange_type === 'binance' && symbols.length === 0) {
        message.warning('Binance 历史回填必须填写交易对，例如 BTCUSDT');
        return false;
      }
      if (endAt.diff(startAt, 'day', true) > 90) {
        message.warning('单次回填时间范围不能超过 90 天');
        return false;
      }
      const response = await backfillExchangeHistory(exchange.id, {
        start_at: startAt.toISOString(),
        end_at: endAt.toISOString(),
        symbols,
      });
      const data = response.data;
      message.success(
        data
          ? `回填完成：拉取订单 ${data.fetched_order_count}、成交 ${data.fetched_fill_count}，已重建完整交易 ${data.closed_trade_count} 笔`
          : '历史回填完成',
        8,
      );
      onSuccess();
      return true;
    }}
  >
    <Alert
      showIcon
      type="info"
      message={
        exchange?.exchange_type === 'binance'
          ? 'Binance 的订单与成交历史接口必须按交易对查询。请填写实际交易过的交易对；回填不会改变日常增量同步游标。'
          : 'OKX 将按账户回填现货和永续历史；交易对可留空。回填不会改变日常增量同步游标。'
      }
      style={{ marginBottom: 20 }}
    />
    <ProFormDateTimeRangePicker
      name="range"
      label="回填时间"
      rules={[{ required: true, message: '请选择回填时间范围' }]}
      fieldProps={{ showTime: true, allowClear: false }}
      width="xl"
    />
    <ProFormTextArea
      name="symbols"
      label="交易对"
      placeholder="BTCUSDT, ETHUSDT"
      fieldProps={{ rows: 3 }}
      extra="支持逗号、空格或换行分隔，最多 20 个；Binance 必填，OKX 可留空。"
    />
  </ModalForm>
);

export default HistoryBackfillForm;
