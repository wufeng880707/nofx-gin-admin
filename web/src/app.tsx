import Footer from "@/components/Footer";
import RightContent from "@/components/RightContent";
import {
  DashboardOutlined,
  DatabaseOutlined,
  FundOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import type {
  Settings as LayoutSettings,
  MenuDataItem,
} from "@ant-design/pro-components";
import { SettingDrawer } from "@ant-design/pro-components";
import type { RunTimeLayoutConfig, RequestConfig } from "@umijs/max";
import { history } from "@umijs/max";
import type { ReactNode } from "react";
import { ConfigProvider } from "antd";
import type { ThemeConfig } from "antd";
import defaultSettings from "../config/defaultSettings";
import { Auth } from "@/utils";
import { getCurrentUser, fetchCurrentMenus } from "@/services/system/login";
import { getFlatMenus, transformRoute } from "@umijs/route-utils";
import routes from "../config/routes";
import { HOST } from "@/services";

const loginPath = "/user/login";
const publicPaths = new Set([loginPath, "/competition"]);
const isPublicPath = (path: string) => publicPaths.has(path);
const layoutSettingsStorageKey = "xtrading-layout-settings";
const menuIcons: Record<string, ReactNode> = {
  dashboard: <DashboardOutlined />,
  data: <DatabaseOutlined />,
  fund: <FundOutlined />,
  setting: <SettingOutlined />,
};

const xtradingTheme: ThemeConfig = {
  token: {
    fontSize: 14,
    fontSizeSM: 12,
    fontSizeLG: 16,
    fontSizeXL: 20,
    fontSizeHeading1: 30,
    fontSizeHeading2: 24,
    fontSizeHeading3: 20,
    fontSizeHeading4: 18,
    fontSizeHeading5: 16,
    lineHeight: 1.5715,
    lineHeightSM: 1.6667,
    lineHeightLG: 1.5,
    lineHeightHeading1: 1.2667,
    lineHeightHeading2: 1.3333,
    lineHeightHeading3: 1.4,
    lineHeightHeading4: 1.4444,
    lineHeightHeading5: 1.5,
  },
};

export function rootContainer(container: ReactNode) {
  return <ConfigProvider theme={xtradingTheme}>{container}</ConfigProvider>;
}

const getLayoutSettings = (): Partial<LayoutSettings> => {
  if (typeof window === "undefined") {
    return defaultSettings;
  }

  try {
    const storedSettings = window.localStorage.getItem(
      layoutSettingsStorageKey,
    );
    return storedSettings
      ? { ...defaultSettings, ...JSON.parse(storedSettings) }
      : defaultSettings;
  } catch {
    return defaultSettings;
  }
};

export const request: RequestConfig = {
  timeout: 60000,
  requestInterceptors: [
    (url, options) => {
      let targetURL = url;
      if (url.startsWith("/api/")) {
        targetURL = `${HOST}${url}`;
      }
      const token = Auth.getToken();
      if (token) {
        const headers = {
          ...options.headers,
          Authorization: `${Auth.getTokenType()} ${token}`,
        };
        return {
          url: targetURL,
          options: { ...options, headers },
        };
      }
      return {
        url: targetURL,
        options,
      };
    },
  ],
};

type InitialStateProps = {
  fetchInitData?: () => Promise<{
    currentUser?: API.User;
    flatMenus?: Record<string, MenuDataItem>;
  }>;
  settings?: Partial<LayoutSettings>;
  routePathCodeMap?: Record<string, string>;
  currentUser?: API.User;
  flatMenus?: Record<string, MenuDataItem>;
  loading?: boolean;
};

/**
 * @see  https://umijs.org/zh-CN/plugins/plugin-initial-state
 * */
export async function getInitialState(): Promise<InitialStateProps> {
  const fetchInitData = async () => {
    const convToMenuItems = (
      menus?: API.Menu[],
      parent_code?: string,
    ): MenuDataItem[] | undefined => {
      if (!menus) {
        return undefined;
      }
      const result: MenuDataItem[] = [];
      menus.forEach((menu) => {
        if (!menu.code) {
          return;
        }
        const code = parent_code
          ? `${parent_code}.${menu.code}`
          : `${menu.code}`;
        const menuItem: MenuDataItem = {
          key: code,
          path: code,
          children: convToMenuItems(menu.children, code),
        };
        result.push(menuItem);
      });
      return result;
    };

    try {
      const userRes = await getCurrentUser();
      const currentUser = userRes.data;
      if (currentUser) {
        currentUser.statusChecked = currentUser?.status === "activated";
      }
      let flatMenus: Record<string, MenuDataItem> = {};
      try {
        const menus = await fetchCurrentMenus();
        if (menus.data) {
          const menuItems = convToMenuItems(menus.data);
          flatMenus = getFlatMenus(menuItems);
        }
        return { currentUser, flatMenus };
      } catch (e) {
        console.error(e);
      }
      return { currentUser, flatMenus };
    } catch (error) {
      if (!isPublicPath(history.location.pathname)) {
        history.push(loginPath);
      }
    }
    return {};
  };

  const flatRouteMenus = getFlatMenus(transformRoute(routes).menuData);
  const routePathCodeMap: Record<string, string> = {};
  Object.keys(flatRouteMenus).forEach((key) => {
    const menu = flatRouteMenus[key];
    let code = menu.code;
    if (menu.pro_layout_parentKeys) {
      const codes: string[] = [];
      menu.pro_layout_parentKeys.forEach((p) => {
        codes.push(flatRouteMenus[p].code);
      });
      if (codes.length > 0) {
        code = `${codes.join(".")}.${code}`;
      }
    }
    routePathCodeMap[menu.path!] = code;
  });

  if (!isPublicPath(history.location.pathname)) {
    const data = await fetchInitData();
    return {
      fetchInitData: fetchInitData,
      settings: getLayoutSettings(),
      routePathCodeMap,
      ...data,
    };
  }

  return {
    fetchInitData: fetchInitData,
    settings: getLayoutSettings(),
    routePathCodeMap,
  };
}

// https://procomponents.ant.design/components/layout
export const layout: RunTimeLayoutConfig = ({
  initialState,
  setInitialState,
}) => {
  const loopMenuItems = (menus?: MenuDataItem[]): MenuDataItem[] => {
    if (!menus || menus.length === 0) {
      return [];
    }

    const result: MenuDataItem[] = [];
    menus.forEach((menu) => {
      if (
        !menu.path ||
        !initialState ||
        !initialState.routePathCodeMap ||
        !initialState.flatMenus
      ) {
        return;
      }

      const code = initialState.routePathCodeMap[menu.path];
      if (code && initialState.flatMenus.hasOwnProperty(code)) {
        const menuItem: MenuDataItem = {
          ...menu,
          icon:
            typeof menu.icon === "string" ? menuIcons[menu.icon] : menu.icon,
        };
        const children = loopMenuItems(menu.children);
        menuItem.children = children;
        result.push(menuItem);
      }
    });
    return result;
  };

  return {
    rightContentRender: () => <RightContent />,
    footerRender: () =>
      history.location.pathname === "/market/chart" ? null : <Footer />,
    onPageChange: () => {
      const { location } = history;
      if (!initialState?.currentUser && !isPublicPath(location.pathname)) {
        history.push(loginPath);
      }
    },
    menu: {
      params: {
        userId: initialState?.currentUser?.id,
      },
      request: async () => {
        const data = loopMenuItems(transformRoute(routes).menuData);
        return data;
      },
    },
    menuHeaderRender: undefined,
    childrenRender: (children) => (
      <>
        {children}
        <SettingDrawer
          disableUrlParams
          enableDarkTheme
          hideCopyButton
          hideHintAlert
          settings={initialState?.settings}
          onSettingChange={(settings) => {
            window.localStorage.setItem(
              layoutSettingsStorageKey,
              JSON.stringify(settings),
            );
            setInitialState((current) => ({
              ...current,
              settings,
            }));
          }}
        />
      </>
    ),
    ...initialState?.settings,
  };
};
