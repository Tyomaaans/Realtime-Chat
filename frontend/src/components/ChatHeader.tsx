export default function ChatHeader() {
  return (
    <header className="page-header">
      <a className="wordmark" href="/" aria-label="Realtime Chat home">
        <svg viewBox="0 0 32 32" width="32" height="32" fill="none" aria-hidden="true">
          <path d="M6 6h20v15H15l-7 5v-5H6V6Z" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" />
          <path d="M11 12h10m-10 4h6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </svg>
        <span>realtime<span className="wordmark-light"> / chat</span></span>
      </a>
      <span className="header-note">One conversation at a time.</span>
    </header>
  );
}
