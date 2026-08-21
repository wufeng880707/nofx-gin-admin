export default [
  {
    path: "/competition",
    layout: false,
    component: "./Competition",
  },
  {
    path: "/user",
    layout: false,
    routes: [
      {
        name: "login",
        path: "/user/login",
        component: "./user/Login",
      },
      {
        component: "./404",
      },
    ],
  },
  {
    path: "/dashboard",
    code: "dashboard",
    name: "dashboard",
    icon: "dashboard",
    routes: [
      {
        path: "/dashboard",
        redirect: "/dashboard/workplace",
      },
      {
        path: "/dashboard/workplace",
        code: "workplace",
        name: "workplace",
        component: "./Welcome",
      },
    ],
  },
  {
    path: "/home",
    redirect: "/dashboard/workplace",
    hideInMenu: true,
  },
  {
    path: "/market",
    code: "market",
    name: "market",
    icon: "stock",
    routes: [
      {
        path: "/market",
        redirect: "/market/chart",
      },
      {
        path: "/market/chart",
        code: "chart",
        name: "chart",
        component: "./market/Chart",
      },
    ],
  },
  {
    path: "/trading",
    code: "trading",
    name: "trading",
    icon: "fund",
    routes: [
      {
        path: "/trading",
        redirect: "/trading/configs",
      },
      {
        path: "/trading/configs",
        code: "configs",
        name: "configs",
        component: "./trading/Configs",
      },
      {
        path: "/trading/strategies",
        code: "strategies",
        name: "strategies",
        component: "./trading/Strategies",
      },
      {
        path: "/trading/spot-strategy",
        code: "spot-strategy",
        name: "spotStrategy",
        component: "./trading/SpotStrategy",
      },
      {
        path: "/trading/notifications",
        code: "notifications",
        name: "notifications",
        component: "./trading/Notifications",
      },
      {
        path: "/trading/traders",
        code: "traders",
        name: "traders",
        component: "./trading/Traders",
      },
      {
        path: "/trading/decisions",
        code: "decisions",
        name: "decisions",
        component: "./trading/Decisions",
      },
      {
        path: "/trading/execution",
        code: "execution",
        name: "execution",
        component: "./trading/Execution",
      },
      {
        path: "/trading/approvals",
        code: "approvals",
        name: "approvals",
        component: "./trading/Approvals",
      },
    ],
  },
  {
    path: "/trading-data",
    code: "trading-data",
    name: "tradingData",
    icon: "data",
    routes: [
      {
        path: "/trading-data",
        redirect: "/trading-data/analytics",
      },
      {
        path: "/trading-data/analytics",
        code: "analytics",
        name: "analytics",
        component: "./trading/Analytics",
      },
      {
        path: "/trading-data/history-sync",
        code: "history-sync",
        name: "historySync",
        component: "./trading/HistorySync",
      },
      {
        path: "/trading-data/assets",
        code: "assets",
        name: "assets",
        component: "./trading/Assets",
      },
      {
        path: "/trading-data/snapshots",
        code: "snapshots",
        name: "snapshots",
        component: "./trading/Snapshots",
      },
      {
        path: "/trading-data/positions",
        code: "positions",
        name: "positions",
        component: "./trading/Positions",
      },
      {
        path: "/trading-data/orders",
        code: "orders",
        name: "orders",
        component: "./trading/Orders",
      },
      {
        path: "/trading-data/fills",
        code: "fills",
        name: "fills",
        component: "./trading/Fills",
      },
      {
        path: "/trading-data/income-events",
        code: "income-events",
        name: "incomeEvents",
        component: "./trading/IncomeEvents",
      },
    ],
  },
  {
    path: "/trading/assets",
    redirect: "/trading-data/assets",
    hideInMenu: true,
  },
  {
    path: "/trading/snapshots",
    redirect: "/trading-data/snapshots",
    hideInMenu: true,
  },
  {
    path: "/trading/positions",
    redirect: "/trading-data/positions",
    hideInMenu: true,
  },
  {
    path: "/trading/orders",
    redirect: "/trading-data/orders",
    hideInMenu: true,
  },
  {
    path: "/trading/fills",
    redirect: "/trading-data/fills",
    hideInMenu: true,
  },
  {
    path: "/trading/income-events",
    redirect: "/trading-data/income-events",
    hideInMenu: true,
  },
  {
    path: "/system",
    code: "system",
    name: "system",
    icon: "setting",
    routes: [
      {
        path: "/system",
        redirect: "/system/menu",
      },
      {
        path: "/system/menu",
        code: "menu",
        name: "menu",
        component: "./system/Menu",
      },
      {
        path: "/system/role",
        code: "role",
        name: "role",
        component: "./system/Role",
      },
      {
        path: "/system/user",
        code: "user",
        name: "user",
        component: "./system/User",
      },
      {
        path: "/system/logger",
        code: "logger",
        name: "logger",
        component: "./system/Logger",
      },
      {
        path: "/system/scheduled-task",
        code: "scheduled-task",
        name: "scheduledTask",
        component: "./system/ScheduledTask",
      },
    ],
  },
  {
    path: "/account/profile",
    name: "account.settings",
    component: "./user/Profile",
    hideInMenu: true,
  },
  {
    path: "/",
    redirect: "/dashboard/workplace",
  },
  {
    component: "./404",
  },
];
