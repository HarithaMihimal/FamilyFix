import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Film, LogIn, Search, Shield, Tv, FolderOpen, Download, Clock3, Play, User, Server } from 'lucide-react';

interface Movie {
  id: number;
  title: string;
  file_name: string;
  file_path: string;
  description: string;
  genre: string;
  format: string;
  file_size: number;
}

interface Answer {
  authenticated?: boolean;
  user?: { username: string; role: string };
  status?: string;
}

const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const power = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** power;
  return `${value.toFixed(1)} ${units[power]}`;
};

function App() {
  const [session, setSession] = useState<Answer | null>(null);
  const [movies, setMovies] = useState<Movie[]>([]);
  const [network, setNetwork] = useState<{ local_server?: string; urls?: string[] } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sessionError, setSessionError] = useState<string | null>(null);
  const [refreshingLibrary, setRefreshingLibrary] = useState(false);
  const [libraryError, setLibraryError] = useState<string | null>(null);
  const [playingMovie, setPlayingMovie] = useState<Movie | null>(null);

  const fetchMovies = async () => {
    const response = await fetch('/api/movies', { credentials: 'include' });
    if (!response.ok) {
      throw new Error(`Could not load movies (server returned ${response.status})`);
    }
    setMovies(await response.json());
    setLibraryError(null);
  };

  const fetchSession = async () => {
    const response = await fetch('/api/session', { credentials: 'include' });
    if (response.status === 401) {
      setSession({ authenticated: false });
      setSessionError(null);
      return;
    }
    if (!response.ok) {
      throw new Error(`Server returned ${response.status} while checking your session`);
    }

    const data = await response.json();
    setSession(data);
    setSessionError(null);
    if (data.authenticated) {
      await fetchMovies();
    }
  };

  const checkSession = async () => {
    setSessionError(null);
    try {
      await fetchSession();
    } catch (err) {
      setSessionError(err instanceof Error ? err.message : 'Could not connect to FamilyFlix');
    }
  };

  useEffect(() => {
    void checkSession();
    fetch('/api/network')
      .then((res) => {
        if (!res.ok) {
          throw new Error(`Network info request failed with status ${res.status}`);
        }
        return res.json();
      })
      .then((data) => setNetwork(data))
      .catch(() => setNetwork({ local_server: 'http://127.0.0.1:3000' }));
  }, []);

  const handleLogin = async (formData: FormData) => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          username: formData.get('username'),
          password: formData.get('password'),
        }),
      });
      if (!response.ok) {
        throw new Error('Invalid username or password');
      }
      await fetchSession();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to sign in');
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = async () => {
    await fetch('/api/logout', { method: 'POST', credentials: 'include' });
    setSession({ authenticated: false });
    setMovies([]);
  };

  const refreshLibrary = async () => {
    setRefreshingLibrary(true);
    try {
      await fetchMovies();
    } catch (err) {
      setLibraryError(err instanceof Error ? err.message : 'Could not refresh the movie library');
    } finally {
      setRefreshingLibrary(false);
    }
  };

  const featured = useMemo(() => movies[0] ?? null, [movies]);

  if (!session) {
    if (sessionError) {
      return (
        <div className="flex min-h-screen items-center justify-center bg-slate-950 px-4 text-slate-200">
          <div className="max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-6 text-center">
            <h1 className="text-xl font-bold text-white">Can’t connect to FamilyFlix</h1>
            <p className="mt-3 text-sm text-slate-300">{sessionError}</p>
            <p className="mt-2 text-sm text-slate-400">Make sure the Go server is running on port 3000, then try again.</p>
            <button
              onClick={() => void checkSession()}
              className="mt-5 rounded-xl bg-cyan-500 px-4 py-2 font-semibold text-slate-950 hover:bg-cyan-400"
            >
              Retry
            </button>
          </div>
        </div>
      );
    }
    return <div className="flex min-h-screen items-center justify-center bg-slate-950 text-slate-200">Loading...</div>;
  }

  if (!session.authenticated) {
    return (
      <LoginScreen onSubmit={handleLogin} loading={loading} error={error} network={network} />
    );
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="border-b border-slate-800 bg-slate-950/80 backdrop-blur">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-3">
            <div className="rounded-xl bg-cyan-500/20 p-2 text-cyan-300"><Film size={22} /></div>
            <div>
              <p className="text-lg font-bold tracking-wide">FamilyFlix</p>
              <p className="text-xs text-slate-400">Private local Wi-Fi library</p>
            </div>
          </div>
          <nav className="hidden items-center gap-5 text-sm text-slate-300 md:flex">
            <a href="#" className="flex items-center gap-2 hover:text-white"><Search size={16} />Home</a>
            <a href="#" className="flex items-center gap-2 hover:text-white"><FolderOpen size={16} />Movies</a>
            <a href="#" className="flex items-center gap-2 hover:text-white"><Clock3 size={16} />Continue Watching</a>
            <a href="#" className="flex items-center gap-2 hover:text-white"><Download size={16} />Downloads</a>
          </nav>
          <button onClick={handleLogout} className="rounded-full border border-slate-700 px-4 py-2 text-sm text-slate-200 hover:border-cyan-500 hover:text-white">
            Sign out
          </button>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-6 py-8">
        <section className="relative overflow-hidden rounded-3xl border border-slate-800 bg-gradient-to-r from-cyan-950/60 via-slate-900 to-slate-950 p-8 shadow-glow">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(34,211,238,0.24),transparent_35%)]" />
          <div className="relative flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div className="max-w-2xl">
              <p className="mb-3 inline-flex items-center gap-2 rounded-full border border-cyan-500/40 bg-cyan-500/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em] text-cyan-300">
                <Shield size={14} /> Featured Library
              </p>
              <h1 className="text-4xl font-black text-white md:text-6xl">{featured?.title ?? 'Your family cinema'}</h1>
              <p className="mt-4 max-w-xl text-sm text-slate-300 md:text-base">
                {featured?.description ?? 'Stream, browse, and share a private movie collection without leaving the local network.'}
              </p>
              <div className="mt-6 flex flex-wrap items-center gap-3">
                <button
                  type="button"
                  disabled={!featured}
                  onClick={() => featured && setPlayingMovie(featured)}
                  className="inline-flex items-center gap-2 rounded-full bg-cyan-500 px-5 py-3 font-semibold text-slate-950 hover:bg-cyan-400 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Play size={16} /> Watch now
                </button>
                <button className="rounded-full border border-slate-700 px-5 py-3 font-semibold text-white hover:border-slate-500">My List</button>
              </div>
            </div>
            <div className="rounded-2xl border border-slate-700 bg-slate-900/70 p-4 text-left backdrop-blur">
              <p className="mb-2 flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-slate-400"><Server size={14} /> Local access</p>
              <p className="text-xl font-bold text-white">{network?.local_server ?? 'http://127.0.0.1:3000'}</p>
              <p className="mt-2 text-sm text-slate-300">Upload progress and streaming stay on your Wi‑Fi network.</p>
            </div>
          </div>
        </section>

        <section className="mt-10 grid gap-6 lg:grid-cols-3">
          <StatCard icon={<Tv size={18} />} title="Movies" value={String(movies.length)} subtitle="Available in the library" />
          <StatCard icon={<User size={18} />} title="Account" value={session.user?.username ?? 'Admin'} subtitle={session.user?.role ?? 'admin'} />
          <StatCard icon={<Download size={18} />} title="Network" value={network?.urls?.length ? String(network.urls.length) : '1'} subtitle="Devices on the local network" />
        </section>

        <section className="mt-10">
          <div className="mb-6 flex items-center justify-between">
            <h2 className="text-2xl font-bold text-white">Recently added</h2>
            <div className="flex items-center gap-4">
              <span className="text-sm text-slate-400">{movies.length} titles</span>
              <button
                type="button"
                onClick={() => void refreshLibrary()}
                disabled={refreshingLibrary}
                className="rounded-full border border-slate-700 px-4 py-2 text-sm text-slate-200 hover:border-cyan-500 disabled:opacity-60"
              >
                {refreshingLibrary ? 'Scanning...' : 'Refresh library'}
              </button>
            </div>
          </div>
          {libraryError && <p role="alert" className="mb-4 text-sm text-red-300">{libraryError}</p>}
          {movies.length === 0 && (
            <p className="mb-5 text-sm text-slate-400">
              No movies found. Add an MP4, MKV, AVI, MOV, or WEBM file under backend/data/movies, then refresh the library.
            </p>
          )}
          <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-4">
            {movies.map((movie) => (
              <div key={movie.id} className="group overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/80 transition hover:-translate-y-1 hover:border-cyan-500/50 hover:shadow-lg hover:shadow-cyan-500/10">
                <div className="aspect-[3/4] bg-gradient-to-br from-slate-700 to-slate-900 p-3">
                  <div className="flex h-full items-end justify-between rounded-xl border border-slate-700 bg-[radial-gradient(circle_at_top,rgba(34,211,238,0.22),transparent_35%),linear-gradient(135deg,#0f172a,#111827)] p-4">
                    <span className="rounded-full bg-slate-950/75 px-2 py-1 text-xs uppercase tracking-wide text-cyan-300">{movie.format}</span>
                    <span className="rounded-full bg-slate-950/75 px-2 py-1 text-xs text-slate-200">{formatBytes(movie.file_size)}</span>
                  </div>
                </div>
                <div className="space-y-3 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <h3 className="text-lg font-semibold text-white">{movie.title}</h3>
                      <p className="text-sm text-slate-400">{movie.genre}</p>
                    </div>
                    <button
                      type="button"
                      onClick={() => setPlayingMovie(movie)}
                      className="rounded-full border border-slate-700 p-2 text-slate-200 hover:border-cyan-500 hover:text-cyan-300"
                      aria-label={`Play ${movie.title}`}
                    >
                      <Play size={15} />
                    </button>
                  </div>
                  <p className="line-clamp-3 text-sm text-slate-400">{movie.description || 'No description yet.'}</p>
                  <div className="flex items-center justify-between text-xs text-slate-400">
                    <span>{movie.file_name}</span>
                    <a href={`/api/movies/${movie.id}/download`} className="font-medium text-cyan-300 hover:text-cyan-200">Download</a>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </section>
      </main>
      {playingMovie && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 p-4" role="dialog" aria-modal="true" aria-labelledby="player-title">
          <div className="w-full max-w-5xl overflow-hidden rounded-2xl border border-slate-700 bg-slate-900 shadow-2xl">
            <div className="flex items-center justify-between gap-4 p-4">
              <h2 id="player-title" className="truncate text-lg font-semibold text-white">{playingMovie.title}</h2>
              <button
                type="button"
                onClick={() => setPlayingMovie(null)}
                className="shrink-0 rounded-lg border border-slate-700 px-3 py-1.5 text-sm text-slate-200 hover:border-cyan-500 hover:text-white"
                aria-label="Close player"
              >
                Close
              </button>
            </div>
            <video
              key={playingMovie.id}
              className="aspect-video w-full bg-black"
              src={`/api/movies/${playingMovie.id}/stream`}
              controls
              autoPlay
              playsInline
              aria-label={playingMovie.title}
            >
              Your browser can’t play this video format. Try downloading the file instead.
            </video>
            <div className="flex justify-end p-4">
              <a href={`/api/movies/${playingMovie.id}/download`} className="font-medium text-cyan-300 hover:text-cyan-200">Download instead</a>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function LoginScreen({ onSubmit, loading, error, network }: { onSubmit: (formData: FormData) => void; loading: boolean; error: string | null; network: { local_server?: string; urls?: string[] } | null;}) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950 px-4 text-slate-100">
      <div className="grid w-full max-w-6xl gap-6 overflow-hidden rounded-3xl border border-slate-800 bg-slate-900/80 shadow-2xl shadow-cyan-950/30 lg:grid-cols-[1.1fr_0.9fr]">
        <div className="relative overflow-hidden bg-gradient-to-br from-cyan-900/60 via-slate-900 to-slate-950 p-8">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(34,211,238,0.2),transparent_40%)]" />
          <div className="relative">
            <div className="inline-flex items-center gap-2 rounded-full border border-cyan-400/40 bg-cyan-500/10 px-3 py-1 text-xs uppercase tracking-[0.2em] text-cyan-300">
              <Film size={14} /> Private streaming
            </div>
            <h1 className="mt-6 text-4xl font-black md:text-5xl">Home cinema for your family.</h1>
            <p className="mt-4 max-w-lg text-slate-300">
              Browse a local movie library, stream directly from your home computer, and share a private collection over the same Wi-Fi network.
            </p>
            <div className="mt-8 rounded-2xl border border-slate-700 bg-slate-950/40 p-4">
              <p className="text-xs uppercase tracking-[0.18em] text-slate-400">Server URL</p>
              <p className="mt-2 font-mono text-lg text-white">{network?.local_server ?? 'http://127.0.0.1:3000'}</p>
              <p className="mt-2 text-sm text-slate-300">Use this address from phones, laptops, and smart TVs on your Wi‑Fi network.</p>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-center p-8">
          <form
            className="w-full max-w-md space-y-5"
            onSubmit={(e) => {
              e.preventDefault();
              const formData = new FormData(e.currentTarget);
              onSubmit(formData);
            }}
          >
            <div>
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300">Welcome back</p>
              <h2 className="mt-2 text-3xl font-bold text-white">Sign in to FamilyFlix</h2>
            </div>

            <label className="block">
              <span className="mb-2 block text-sm text-slate-300">Username</span>
              <input name="username" defaultValue="admin" className="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-3 text-white outline-none ring-0 placeholder:text-slate-500 focus:border-cyan-500" placeholder="admin" />
            </label>

            <label className="block">
              <span className="mb-2 block text-sm text-slate-300">Password</span>
              <input type="password" name="password" defaultValue="admin123" className="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-3 text-white placeholder:text-slate-500 focus:border-cyan-500" placeholder="••••••••" />
            </label>

            {error && <p className="rounded-xl border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">{error}</p>}

            <button type="submit" disabled={loading} className="w-full rounded-xl bg-cyan-500 px-4 py-3 font-semibold text-slate-950 transition hover:bg-cyan-400 disabled:cursor-not-allowed disabled:opacity-70">
              {loading ? 'Signing in...' : 'Sign in'}
            </button>

            <div className="flex items-center justify-between border-t border-slate-800 pt-4 text-xs text-slate-400">
              <span>Default admin</span>
              <span>admin / admin123</span>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

function StatCard({ icon, title, value, subtitle }: { icon: ReactNode; title: string; value: string; subtitle: string }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-900/80 p-5">
      <div className="mb-4 inline-flex rounded-xl border border-cyan-500/30 bg-cyan-500/10 p-2 text-cyan-300">{icon}</div>
      <p className="text-sm text-slate-400">{title}</p>
      <p className="mt-2 text-3xl font-bold text-white">{value}</p>
      <p className="mt-2 text-sm text-slate-500">{subtitle}</p>
    </div>
  );
}

export default App;
