import { ModalForm, ProFormSelect } from '@ant-design/pro-components';
import { Alert, message } from 'antd';
import React from 'react';
import { replaceExchangeHistorySymbols } from '@/services/trading/catalog';

type Props = {
  status?: TradingAPI.HistorySyncStatus;
  visible: boolean;
  onCancel: () => void;
  onSuccess: () => void;
};

type Values = { symbols?: string[] };

const normalizeSymbols = (values?: string[]) =>
  Array.from(
    new Set(
      (values ?? [])
        .flatMap((value) => value.split(/[\s,;]+/))
        .map((value) => value.trim().toUpperCase())
        .filter(Boolean),
    ),
  );

const HistorySymbolsForm: React.FC<Props> = ({ status, visible, onCancel, onSuccess }) => (
  <ModalForm<Values>
    visible={visible}
    title={`历史交易对${status ? ` - ${status.exchange_name}` : ''}`}
    width={620}
    initialValues={{ symbols: status?.registered_symbols ?? [] }}
    modalProps={{ destroyOnClose: true, maskClosable: false, onCancel }}
    submitter={{ searchConfig: { submitText: '保存', resetText: '取消' } }}
    onFinish={async (values) => {
      if (!status) return false;
      const symbols = normalizeSymbols(values.symbols);
      await replaceExchangeHistorySymbols(status.exchange_id, symbols);
      message.success(`已保存 ${symbols.length} 个历史交易对`);
      onSuccess();
      return true;
    }}
  >
    <Alert
      showIcon
      type="info"
      message="Binance 历史订单接口必须指定交易对。这里登记的币种会用于日常增量同步，已清仓且无挂单的币种也能继续回补。"
      style={{ marginBottom: 20 }}
    />
    <ProFormSelect
      name="symbols"
      label="交易对"
      mode="tags"
      placeholder="输入 BTCUSDT 后按回车"
      fieldProps={{ tokenSeparators: [',', ';', ' '], maxTagCount: 'responsive' }}
      extra="最多 50 个；留空并保存会清除注册表，但可能造成历史覆盖不完整。"
    />
  </ModalForm>
);

export default HistorySymbolsForm;
