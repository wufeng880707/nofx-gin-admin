import React from 'react';
import {
  PlusOutlined,
  PlusCircleOutlined,
  DeleteOutlined,
  EditOutlined,
  QuestionCircleOutlined,
  UploadOutlined,
  ImportOutlined,
} from '@ant-design/icons';
import { Button, Tooltip, Popconfirm } from 'antd';
import type { ButtonProps, PopconfirmProps } from 'antd';
import { useLocation, useModel, useIntl } from '@umijs/max';

export type AddButtonProps = ButtonProps & { code: string };

const AddButton: React.FC<AddButtonProps> = (props) => {
  const { initialState } = useModel('@@initialState');
  const intl = useIntl();
  const { pathname } = useLocation();
  const code = initialState!.routePathCodeMap![pathname];
  const show = initialState!.flatMenus!.hasOwnProperty(`${code}.${props.code}`);
  return (
    <>
      {show && (
        <Button icon={<PlusOutlined />} type="primary" {...props} title={undefined}>
          {props.title ? props.title : intl.formatMessage({ id: 'button.add' })}
        </Button>
      )}
    </>
  );
};
export type AddIconButtonProps = ButtonProps & { code: string };

const AddIconButton: React.FC<AddIconButtonProps> = (props) => {
  const { initialState } = useModel('@@initialState');
  const intl = useIntl();
  const { pathname } = useLocation();
  const code = initialState!.routePathCodeMap![pathname];
  const show = initialState!.flatMenus!.hasOwnProperty(`${code}.${props.code}`);

  return (
    <>
      {show && (
        <Tooltip
          title={
            props.title
              ? props.title
              : intl.formatMessage({ id: 'button.add', defaultMessage: '添加' })
          }
        >
          <Button
            size="middle"
            shape="circle"
            type="link"
            icon={<PlusCircleOutlined />}
            style={{ border: 0 }}
            {...props}
            title={undefined}
          />
        </Tooltip>
      )}
    </>
  );
};

export type DelIconButtonProps = {
  buttonProps?: ButtonProps;
} & PopconfirmProps & { code: string };

const DelIconButton: React.FC<DelIconButtonProps> = (props) => {
  const { initialState } = useModel('@@initialState');
  const { pathname } = useLocation();
  const intl = useIntl();
  const code = initialState!.routePathCodeMap![pathname];
  const show = initialState!.flatMenus!.hasOwnProperty(`${code}.${props.code}`);
  const handleConfirm: PopconfirmProps['onConfirm'] = async (event) => {
    try {
      await props.onConfirm?.(event);
    } catch {
      // The shared request handler already reports the API error to the user.
    }
  };

  return (
    <>
      {show && (
        <Popconfirm
          {...props}
          key="delete"
          title={props.title}
          icon={<QuestionCircleOutlined style={{ color: 'red' }} />}
          placement="topLeft"
          onCancel={(e) => e!.stopPropagation()}
          onConfirm={handleConfirm}
        >
          <Tooltip
            title={
              props.buttonProps?.title
                ? props.buttonProps.title
                : intl.formatMessage({ id: 'button.delete', defaultMessage: '删除' })
            }
          >
            <Button
              size="middle"
              shape="circle"
              danger
              icon={<DeleteOutlined />}
              style={{ border: 0 }}
              onClick={(e) => e.stopPropagation()}
              {...props.buttonProps}
              title={undefined}
            />
          </Tooltip>
        </Popconfirm>
      )}
    </>
  );
};

export type EditIconButtonProps = {
  icon?: React.ReactNode;
} & ButtonProps & { code: string };

const EditIconButton: React.FC<EditIconButtonProps> = (props) => {
  const { initialState } = useModel('@@initialState');
  const intl = useIntl();
  const { pathname } = useLocation();
  const code = initialState!.routePathCodeMap![pathname];
  const show = initialState!.flatMenus!.hasOwnProperty(`${code}.${props.code}`);

  return (
    <>
      {show && (
        <Tooltip
          title={
            props.title
              ? props.title
              : intl.formatMessage({ id: 'button.edit', defaultMessage: '编辑' })
          }
        >
          <Button
            size="middle"
            shape="circle"
            type="link"
            icon={props.icon ? props.icon : <EditOutlined />}
            style={{ border: 0 }}
            {...props}
            title={undefined}
          />
        </Tooltip>
      )}
    </>
  );
};

export type ActionIconButtonProps = ButtonProps & {
  code: string;
  tooltip: string;
  confirmTitle?: string;
};

const ActionIconButton: React.FC<ActionIconButtonProps> = ({
  code,
  tooltip,
  confirmTitle,
  onClick,
  ...buttonProps
}) => {
  const { initialState } = useModel('@@initialState');
  const { pathname } = useLocation();
  const routeCode = initialState?.routePathCodeMap?.[pathname];
  const show = Boolean(
    routeCode && initialState?.flatMenus?.hasOwnProperty(`${routeCode}.${code}`),
  );
  if (!show) return null;

  const button = (
    <Tooltip title={tooltip}>
      <Button
        size="middle"
        shape="circle"
        type="link"
        style={{ border: 0 }}
        onClick={confirmTitle ? (event) => event.stopPropagation() : onClick}
        {...buttonProps}
      />
    </Tooltip>
  );
  if (!confirmTitle) return button;
  const handleConfirm: PopconfirmProps['onConfirm'] = async (event) => {
    try {
      await (onClick as PopconfirmProps['onConfirm'])?.(event);
    } catch {
      // Request errors are displayed by the shared request handler. Resolve here
      // so Ant Design closes the confirmation overlay after a failed action.
    }
  };
  return (
    <Popconfirm title={confirmTitle} onConfirm={handleConfirm}>
      {button}
    </Popconfirm>
  );
};
export type ExportButtonProps = ButtonProps & { code: string };

const ExportButton: React.FC<ExportButtonProps> = (props) => {
  const { initialState } = useModel('@@initialState');
  const intl = useIntl();
  const { pathname } = useLocation();
  const code = initialState!.routePathCodeMap![pathname];
  const show = initialState!.flatMenus!.hasOwnProperty(`${code}.${props.code}`);

  return (
    <>
      {show && (
        <Button icon={<UploadOutlined />} type="primary" {...props} title={undefined}>
          {props.title
            ? props.title
            : intl.formatMessage({ id: 'button.export', defaultMessage: '导出' })}
        </Button>
      )}
    </>
  );
};
export type ImportButtonProps = ButtonProps & { code: string };

const ImportButton: React.FC<ImportButtonProps> = (props) => {
  const { initialState } = useModel('@@initialState');
  const intl = useIntl();
  const { pathname } = useLocation();
  const code = initialState!.routePathCodeMap![pathname];
  const show = initialState!.flatMenus!.hasOwnProperty(`${code}.${props.code}`);

  return (
    <>
      {show && (
        <Button icon={<ImportOutlined />} type="primary" {...props} title={undefined}>
          {props.title
            ? props.title
            : intl.formatMessage({ id: 'button.import', defaultMessage: '导入' })}
        </Button>
      )}
    </>
  );
};

export {
  ActionIconButton,
  AddButton,
  AddIconButton,
  DelIconButton,
  EditIconButton,
  ExportButton,
  ImportButton,
};
