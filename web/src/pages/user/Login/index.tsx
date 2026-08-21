import { LockOutlined, UserOutlined, SafetyOutlined } from '@ant-design/icons';
import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { message, Col, Row, Button } from 'antd';
import React, { useState, useEffect, useRef } from 'react';
import Settings from '../../../../config/defaultSettings';
import { useIntl, SelectLang, Helmet } from '@umijs/max';
import { getCaptchaId, getCaptchaImageURL, login } from '@/services/system/login';
import styles from './index.less';
import { Util, Auth } from '@/utils';
import type { ProFormInstance } from '@ant-design/pro-components';
import { HOST } from '@/services';
import Footer from '@/components/Footer';

const Login: React.FC = () => {
  const intl = useIntl();
  const formRef = useRef<ProFormInstance<API.LoginForm>>(null);
  const [captchaID, setCaptchaID] = useState('');
  const [captchaURL, setCaptchaURL] = useState('');

  const fetchCaptchaID = () => {
    getCaptchaId().then((res?) => {
      if (res && res.data && res.data.captcha_id) {
        const cid = res.data.captcha_id;
        setCaptchaID(cid);
        setCaptchaURL(getCaptchaImageURL(cid));
      }
    });
  };

  useEffect(() => {
    fetchCaptchaID();
  }, []);

  const getPostLoginPath = () => {
    const fallback = '/dashboard/workplace';
    const redirect = new URLSearchParams(window.location.search).get('redirect');
    if (!redirect || !redirect.startsWith('/') || redirect.startsWith('//')) {
      return fallback;
    }

    const target = new URL(redirect, window.location.origin);
    if (target.origin !== window.location.origin || target.pathname.startsWith('/user/login')) {
      return fallback;
    }
    return `${target.pathname}${target.search}${target.hash}`;
  };

  const handleSubmit = async (values: API.LoginForm) => {
    let errMsg = intl.formatMessage({ id: 'pages.login.failure', defaultMessage: '登录失败' });

    try {
      values.captcha_id = captchaID;
      const result = await login({ ...values });
      if (result.success) {
        message.success(
          intl.formatMessage({ id: 'pages.login.success', defaultMessage: '登录成功' }),
        );
        Util.setUsername(values.username);

        if (result.data?.access_token) {
          Auth.setToken(
            result.data?.access_token,
            result.data?.token_type,
            result.data?.expires_at,
          );
        }
        // A full navigation reloads the latest application bundle and lets
        // getInitialState fetch the user and menus from the stored token.
        window.location.replace(getPostLoginPath());
        return;
      }

      if (result.error) {
        errMsg = result.error?.detail || '';
      }
      message.error(errMsg);
    } catch (error) {
      const e = error as { data: API.ResponseResult<any> };
      if (e.data.error) {
        errMsg = e.data.error?.detail || '';
      }
      message.error(errMsg);
    }
    formRef.current?.resetFields(['captcha_code']);
    fetchCaptchaID();
  };

  return (
    <div className={styles.container}>
      <Helmet>
        <title>
          {intl.formatMessage({
            id: 'menu.login',
            defaultMessage: '登录',
          })}
          - {Settings.title}
        </title>
      </Helmet>
      <div className={styles.lang} data-lang>
        {SelectLang && <SelectLang />}
      </div>
      <div className={styles.content}>
        <LoginForm<API.LoginForm>
          formRef={formRef}
          logo={<img alt="logo" src="/logo.svg" />}
          title={Settings.title?.toString()}
          subTitle={intl.formatMessage({ id: 'pages.layouts.userLayout.title' })}
          actions={null}
          submitter={{
            searchConfig: {
              submitText: intl.formatMessage({ id: 'pages.login.submit', defaultMessage: '登录' }),
            },
          }}
          onFinish={async (values: API.LoginForm) => {
            await handleSubmit(values);
          }}
        >
          <ProFormText
            name="username"
            fieldProps={{
              size: 'large',
              prefix: <UserOutlined className={styles.prefixIcon} />,
            }}
            placeholder={intl.formatMessage({
              id: 'pages.login.username.placeholder',
              defaultMessage: '请输入用户名',
            })}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'pages.login.username.required',
                  defaultMessage: '请输入用户名!',
                }),
              },
            ]}
          />
          <ProFormText.Password
            name="password"
            fieldProps={{
              size: 'large',
              prefix: <LockOutlined className={styles.prefixIcon} />,
            }}
            placeholder={intl.formatMessage({
              id: 'pages.login.password.placeholder',
              defaultMessage: '请输入密码',
            })}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'pages.login.password.required',
                  defaultMessage: '请输入密码！',
                }),
              },
            ]}
          />

          <Row justify={'space-between'} gutter={8}>
            <Col span={17}>
              <ProFormText
                fieldProps={{
                  size: 'large',
                  prefix: <SafetyOutlined />,
                }}
                placeholder={intl.formatMessage({
                  id: 'pages.login.captcha.placeholder',
                  defaultMessage: '请输入验证码',
                })}
                name="captcha_code"
                rules={[
                  {
                    required: true,
                    message: intl.formatMessage({
                      id: 'pages.login.captcha.required',
                      defaultMessage: '请输入验证码！',
                    }),
                  },
                ]}
              />
            </Col>
            <Col span={7}>
              <Button
                loading={captchaURL ? false : true}
                style={{ height: 40, width: '100%', padding: 0, overflow: 'hidden' }}
                onClick={() => {
                  setCaptchaURL(getCaptchaImageURL(captchaID));
                }}
              >
                {captchaURL && (
                  <img
                    src={`${HOST}${captchaURL}`}
                    alt=""
                    style={{ width: '100%', height: '100%' }}
                  />
                )}
              </Button>
            </Col>
          </Row>
        </LoginForm>
      </div>
      <Footer />
    </div>
  );
};

export default Login;
