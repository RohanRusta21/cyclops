// src/components/auth/PublicRoute.tsx
import { Navigate, Outlet } from "react-router-dom";
import { useEffect, useState } from "react";

const PublicRoute: React.FC = () => {
  const [isAuthenticated, setIsAuthenticated] = useState<boolean | null>(null);

  useEffect(() => {
    const loggedIn = localStorage.getItem("isLoggedIn") === "true";
    setIsAuthenticated(loggedIn);
  }, []);

  if (isAuthenticated === null) {
    return <div>Loading...</div>;
  }

  return !isAuthenticated ? <Outlet /> : <Navigate to="/" replace />;
};

export default PublicRoute;
