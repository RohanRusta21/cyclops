// import { createBrowserRouter, RouterProvider } from "react-router-dom";
// import routes from "./routes";
// import Page404 from "./components/pages/Page404";
// import AppLayout from "./components/layouts/AppLayout";
// import { ThemeProvider } from "./components/theme/ThemeContext";

// export default function App() {
//   const router = createBrowserRouter([
//     {
//       element: (
//         <ThemeProvider>
//           <AppLayout />
//         </ThemeProvider>
//       ),
//       errorElement: <Page404 />,
//       children: routes,
//     },
//   ]);

//   return <RouterProvider router={router} />;
// }

// src/App.tsx
// src/App.tsx
import {
  createBrowserRouter,
  RouterProvider,
  Navigate,
} from "react-router-dom"; // ✅ Navigate imported
import routes from "./routes";
import Page404 from "./components/pages/Page404";
import AppLayout from "./components/layouts/AppLayout";
import { ThemeProvider } from "./components/theme/ThemeContext";
import Login from "./components/Login/Login"; // ✅ Ensure folder is named "login" (lowercase)
import ProtectedRoute from "./components/auth/ProtectedRoute";
import PublicRoute from "./components/auth/PublicRoute";

export default function App() {
  const router = createBrowserRouter([
    // Public Routes (e.g., Login)
    {
      element: <PublicRoute />, // ✅ Self-closing, no children passed here
      children: [
        {
          path: "/login",
          element: (
            <Login
              onLogin={() => {
                localStorage.setItem("isLoggedIn", "true");
                window.location.href = "/";
              }}
            />
          ),
        },
      ],
    },

    // Protected Routes (App Layout + your pages)
    {
      element: <ProtectedRoute />, // ✅ Self-closing
      children: [
        {
          element: (
            <ThemeProvider>
              <AppLayout />
            </ThemeProvider>
          ),
          errorElement: <Page404 />,
          children: routes, // ✅ All your existing routes are protected
        },
      ],
    },

    // Catch-all: redirect everything else to login
    {
      path: "*",
      element: <Navigate to="/login" replace />,
    },
  ]);

  return <RouterProvider router={router} />;
}
