import { Link } from 'react-router-dom';

/** The shared Not Found page, for unknown URLs and for 404s on a page's main resource. */
export function NotFoundPage() {
  return (
    <>
      <h1>Page not found</h1>
      <p className="lead">There is nothing at this address, or you do not have access to it.</p>
      <p><Link to="/">Back to the dashboard</Link></p>
    </>
  );
}
