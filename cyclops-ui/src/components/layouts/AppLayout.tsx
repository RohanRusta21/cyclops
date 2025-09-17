// src/components/layouts/AppLayout.tsx
import { Outlet } from "react-router-dom";
import SideNav from "./Sidebar";
import React, { Suspense } from "react";
import Sider from "antd/es/layout/Sider";
import { Content, Header } from "antd/es/layout/layout";
import { ConfigProvider, Layout, theme, Button } from "antd"; // 👈 Added Button
import { useTheme } from "../theme/ThemeContext";
import { ThemeSwitch } from "../theme/ThemeSwitch";
import { useNavigate } from "react-router-dom"; // 👈 For SPA navigation

export default function AppLayout() {
  const { mode } = useTheme();
  const navigate = useNavigate();

  const handleLogout = () => {
    localStorage.removeItem("isLoggedIn");
    navigate("/login", { replace: true });
  };

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <ConfigProvider
        theme={{
          token: {
            colorPrimary: "#fe8801",
          },
          algorithm:
            mode === "light" ? theme.defaultAlgorithm : theme.darkAlgorithm,
        }}
      >
        <Sider
          width={200}
          style={{
            position: "fixed",
            height: "100%",
          }}
        >
          <SideNav />
        </Sider>
        <Layout style={{ marginLeft: 200 }}>
          <Header
            style={{
              position: "fixed",
              top: 0,
              left: 200,
              right: 0,
              zIndex: 100,
              display: "flex",
              justifyContent: "flex-end", // 👈 Align items to right
              alignItems: "center", // 👈 Vertically center
              padding: "0 16px",
              height: 64,
              background: mode === "light" ? "#fff" : "#141414",
              borderBottom: `1px solid ${mode === "light" ? "#f0f0f0" : "#303030"}`,
            }}
          >
            {/* Theme Switch + Logout Button */}
            <div style={{ display: "flex", gap: "12px", alignItems: "center" }}>
              <ThemeSwitch />
              <Button type="primary" danger onClick={handleLogout}>
                Logout
              </Button>
            </div>
          </Header>

          <Content
            style={{
              marginTop: 64,
              padding: 24,
              minHeight: "calc(100vh - 112px)",
              background: mode === "light" ? "#fff" : "#141414",
            }}
          >
            <Suspense
              fallback={<h1 style={{ textAlign: "center" }}>Loading...</h1>}
            >
              <Outlet />
            </Suspense>
          </Content>
        </Layout>
      </ConfigProvider>
    </Layout>
  );
}
