import { Routes, Route } from "react-router-dom";
import Layout from "./components/Layout";
import Dashboard from "./pages/Dashboard";
import Projects from "./pages/Projects";
import GitPage from "./pages/GitPage";
import TerminalPage from "./pages/TerminalPage";
import DatabasePage from "./pages/DatabasePage";
import ApiPage from "./pages/ApiPage";
import EnvPage from "./pages/EnvPage";
import SnippetsPage from "./pages/SnippetsPage";
import TasksPage from "./pages/TasksPage";
import ProcessesPage from "./pages/ProcessesPage";
import ContainersPage from "./pages/ContainersPage";
import RemotePage from "./pages/RemotePage";
import PluginsPage from "./pages/PluginsPage";
import SettingsPage from "./pages/SettingsPage";

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/projects" element={<Projects />} />
        <Route path="/git" element={<GitPage />} />
        <Route path="/terminal" element={<TerminalPage />} />
        <Route path="/database" element={<DatabasePage />} />
        <Route path="/api" element={<ApiPage />} />
        <Route path="/env" element={<EnvPage />} />
        <Route path="/snippets" element={<SnippetsPage />} />
        <Route path="/tasks" element={<TasksPage />} />
        <Route path="/processes" element={<ProcessesPage />} />
        <Route path="/containers" element={<ContainersPage />} />
        <Route path="/remote" element={<RemotePage />} />
        <Route path="/plugins" element={<PluginsPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="*" element={<Dashboard />} />
      </Route>
    </Routes>
  );
}
