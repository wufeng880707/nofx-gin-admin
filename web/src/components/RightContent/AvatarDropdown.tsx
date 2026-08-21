import { logout } from '@/services/system/login';
import { Auth } from '@/utils';
import { LogoutOutlined, UserOutlined } from '@ant-design/icons';
import { Avatar, Menu, Spin } from 'antd';
import type { MenuProps } from 'antd';
import React, { useCallback } from 'react';
import { history, useModel, useIntl } from '@umijs/max';
import HeaderDropdown from '../HeaderDropdown';
import styles from './index.less';

export type GlobalHeaderRightProps = {
  menu?: boolean;
};

const loginOut = () => {
  logout()
    .then(() => {
      Auth.removeToken();
    })
    .catch(() => {
      Auth.removeToken();
    });
  const { search, pathname } = history.location;
  const redirect = new URLSearchParams(search).get('redirect');
  // Note: There may be security issues, please note
  if (window.location.pathname !== '/user/login' && !redirect) {
    history.replace({
      pathname: '/user/login',
      search: new URLSearchParams({ redirect: pathname + search }).toString(),
    });
  }
};

const AvatarDropdown: React.FC<GlobalHeaderRightProps> = ({ menu }) => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const intl = useIntl();

  const onMenuClick = useCallback(
    (event: Parameters<NonNullable<MenuProps['onClick']>>[0]) => {
      const { key } = event;
      if (key === 'logout') {
        setInitialState((s) => ({ ...s, currentUser: undefined }));
        loginOut();
        return;
      }
      if (key === 'profile') {
        history.push(`/account/profile`);
      }
    },
    [setInitialState],
  );

  const loading = (
    <span className={`${styles.action} ${styles.account}`}>
      <Spin
        size="small"
        style={{
          marginLeft: 8,
          marginRight: 8,
        }}
      />
    </span>
  );

  if (!initialState) {
    return loading;
  }

  const { currentUser } = initialState;

  if (!currentUser || !currentUser.username) {
    return loading;
  }

  const menuItems: MenuProps['items'] = [
    ...(menu
      ? [
          {
            key: 'profile',
            icon: <UserOutlined />,
            label: intl.formatMessage({
              id: 'component.RightContent.profile',
              defaultMessage: '个人设置',
            }),
          },
          {
            type: 'divider' as const,
          },
        ]
      : []),
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: intl.formatMessage({
        id: 'component.RightContent.logout',
        defaultMessage: '退出登录',
      }),
    },
  ];

  const menuHeaderDropdown = (
    <Menu className={styles.menu} selectedKeys={[]} onClick={onMenuClick} items={menuItems} />
  );

  return (
    <HeaderDropdown overlay={menuHeaderDropdown}>
      <span className={`${styles.action} ${styles.account}`}>
        <Avatar
          size="small"
          style={{ backgroundColor: '#ffbf00', marginRight: 8 }}
          icon={<UserOutlined />}
        />
        <span className={`${styles.name} anticon`}>{currentUser.name}</span>
      </span>
    </HeaderDropdown>
  );
};

export default AvatarDropdown;
