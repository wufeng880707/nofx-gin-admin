import { Dropdown } from 'antd';
import type { DropdownProps } from 'antd';
import classNames from 'classnames';
import React from 'react';
import styles from './index.less';

export type HeaderDropdownProps = {
  overlayClassName?: string;
  overlay: React.ReactNode;
  placement?: 'bottomLeft' | 'bottomRight' | 'topLeft' | 'topCenter' | 'topRight' | 'bottomCenter';
} & Omit<DropdownProps, 'dropdownRender'>;

const HeaderDropdown: React.FC<HeaderDropdownProps> = ({
  overlayClassName: cls,
  overlay,
  ...restProps
}) => (
  <Dropdown
    overlayClassName={classNames(styles.container, cls)}
    dropdownRender={() => overlay}
    {...restProps}
  />
);

export default HeaderDropdown;
