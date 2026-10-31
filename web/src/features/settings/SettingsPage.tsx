import { NavLink, Outlet } from 'react-router-dom';

/** The settings area at `/settings/*`: a section menu beside the selected section. */
export function SettingsPage() {
  return (
    <>
      <h1>Settings</h1>
      <div className="settings">
        <nav className="settings-nav" aria-label="Settings sections">
          <NavLink to="/settings" end>Profile</NavLink>
          <NavLink to="/settings/password">Password</NavLink>
          <NavLink to="/settings/tokens">Access tokens</NavLink>
          <NavLink to="/settings/sessions">Sessions</NavLink>
        </nav>
        <div className="settings-body">
          <Outlet />
        </div>
      </div>
    </>
  );
}
