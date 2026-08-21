import { defineConfig } from "@umijs/max";

import defaultSettings from "./defaultSettings";
import proxy from "./proxy";
import routes from "./routes";

const { REACT_APP_ENV } = process.env;

export default defineConfig({
  hash: true,
  antd: {},
  access: {},
  initialState: {},
  model: {},
  request: {},
  layout: {
    // https://umijs.org/zh-CN/plugins/plugin-layout
    locale: true,
    siderWidth: 218,
    ...defaultSettings,
  },
  // https://umijs.org/zh-CN/plugins/plugin-locale
  locale: {
    // default zh-CN
    default: "en-US",
    antd: true,
    // default true, when it is true, will use `navigator.language` overwrite default
    baseNavigator: true,
  },
  routes,
  title: false,
  ignoreMomentLocale: true,
  proxy: proxy[REACT_APP_ENV || "dev"],
  manifest: {
    basePath: "/",
  },
  fastRefresh: true,
  esbuildMinifyIIFE: true,
  mfsu: {},
});
