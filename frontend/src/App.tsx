import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Film, Search, Shield, Tv, FolderOpen, Download, Clock3, Play, User, Server, QrCode, UserPlus, Trash2 } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';

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

interface WatchProgress {
  movie_id: number;
  user_id: number;
  position: number;
  updated_at: string;
}

interface Answer {
  authenticated?: boolean;
  user?: { username: string; role: string };
  status?: string;
}

interface Account {
  id: number;
  username: string;
  role: string;
  created_at: string;
}

const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const power = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** power;
  return `${value.toFixed(1)} ${units[power]}`;
};

function App() {
  const accessHost = '192.168.8.173';
  const accessPort = '5173';
  const accessURL = `http://${accessHost}:${accessPort}`;
  const accessAddress = `${accessHost}:${accessPort}`;
  const [session, setSession] = useState<Answer | null>(null);
  const [movies, setMovies] = useState<Movie[]>([]);
  const [myList, setMyList] = useState<Movie[]>([]);
  const [watchProgress, setWatchProgress] = useState<WatchProgress[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountError, setAccountError] = useState<string | null>(null);
  const [accountNotice, setAccountNotice] = useState<string | null>(null);
  const [accountBusy, setAccountBusy] = useState(false);
  const [network, setNetwork] = useState<{ local_server?: string; urls?: string[]; active_devices?: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sessionError, setSessionError] = useState<string | null>(null);
  const [refreshingLibrary, setRefreshingLibrary] = useState(false);
  const [libraryError, setLibraryError] = useState<string | null>(null);
  const [playingMovie, setPlayingMovie] = useState<Movie | null>(null);
  const [myListBusy, setMyListBusy] = useState(false);
  const lastProgressSaveRef = useRef<Record<number, number>>({});

  const fetchMovies = async () => {
    const response = await fetch('/api/movies', { credentials: 'include' });
    if (!response.ok) {
      throw new Error(`Could not load movies (server returned ${response.status})`);
    }
    setMovies(await response.json());
    setLibraryError(null);
  };

  const fetchMyList = async () => {
    try {
      const response = await fetch('/api/my-list', { credentials: 'include' });
      if (!response.ok) {
        throw new Error(`Could not load My List (server returned ${response.status})`);
      }
      const data = await response.json();
      setMyList(Array.isArray(data) ? data : []);
    } catch {
      setMyList([]);
    }
  };

  const fetchWatchProgress = async () => {
    try {
      const response = await fetch('/api/progress', { credentials: 'include' });
      if (!response.ok) {
        throw new Error(`Could not load watch progress (server returned ${response.status})`);
      }
      const data = await response.json();
      setWatchProgress(Array.isArray(data) ? data : []);
    } catch {
      setWatchProgress([]);
    }
  };

  const fetchAccounts = async () => {
    const response = await fetch('/api/admin/users', { credentials: 'include' });
    if (!response.ok) {
      throw new Error(`Could not load user accounts (server returned ${response.status})`);
    }
    const data = await response.json();
    setAccounts(Array.isArray(data) ? data : []);
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
      const tasks = [fetchMovies(), fetchMyList(), fetchWatchProgress()];
      if (data.user?.role === 'admin') {
        tasks.push(fetchAccounts());
      } else {
        setAccounts([]);
      }
      await Promise.all(tasks);
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

    let active = true;
    const fetchNetwork = async () => {
      try {
        const response = await fetch('/api/network');
        if (!response.ok) {
          throw new Error(`Network info request failed with status ${response.status}`);
        }
        const data = await response.json();
        if (active) {
          setNetwork(data);
        }
      } catch {
        if (active) {
          setNetwork(null);
        }
      }
    };

    void fetchNetwork();
    const interval = window.setInterval(() => void fetchNetwork(), 15000);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
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
    setMyList([]);
    setWatchProgress([]);
    setAccounts([]);
    lastProgressSaveRef.current = {};
  };

  const createAccount = async (formData: FormData) => {
    setAccountBusy(true);
    setAccountError(null);
    setAccountNotice(null);
    try {
      const response = await fetch('/api/admin/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          username: formData.get('username'),
          password: formData.get('password'),
        }),
      });
      if (!response.ok) {
        const detail = await response.text();
        throw new Error(detail.trim() || `Could not create account (server returned ${response.status})`);
      }
      await fetchAccounts();
      setAccountNotice('Normal user account created.');
      return true;
    } catch (err) {
      setAccountError(err instanceof Error ? err.message : 'Could not create account');
      return false;
    } finally {
      setAccountBusy(false);
    }
  };

  const deleteAccount = async (account: Account) => {
    if (!window.confirm(`Delete the normal user account "${account.username}"?`)) return;
    setAccountBusy(true);
    setAccountError(null);
    setAccountNotice(null);
    try {
      const response = await fetch(`/api/admin/users/${account.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (!response.ok) {
        throw new Error(`Could not delete account (server returned ${response.status})`);
      }
      await fetchAccounts();
      setAccountNotice(`Account "${account.username}" deleted.`);
    } catch (err) {
      setAccountError(err instanceof Error ? err.message : 'Could not delete account');
    } finally {
      setAccountBusy(false);
    }
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
  const isMovieInMyList = (movieId: number) => myList.some((movie) => movie.id === movieId);
  const featuredInMyList = useMemo(
    () => (featured ? isMovieInMyList(featured.id) : false),
    [featured, myList],
  );
  const continueWatchingMovies = useMemo(() => {
    const progressByMovieId = new Map(watchProgress.map((item) => [item.movie_id, item.position]));
    return movies.filter((movie) => (progressByMovieId.get(movie.id) ?? 0) > 0);
  }, [movies, watchProgress]);

  const getSavedPosition = (movieId: number) => watchProgress.find((item) => item.movie_id === movieId)?.position ?? 0;

  const scrollToSection = (id: string) => {
    const target = document.getElementById(id);
    if (target) {
      target.scrollIntoView({ behavior: 'smooth', block: 'start' });
    } else if (id === 'home') {
      window.scrollTo({ top: 0, behavior: 'smooth' });
    }
  };

  const saveMovieProgress = async (movieId: number, position: number, force = false) => {
    if (!Number.isFinite(position) || position < 0) return;
    const roundedPosition = Math.floor(position);
    const lastSaved = lastProgressSaveRef.current[movieId] ?? -1;
    if (!force && Math.abs(roundedPosition - lastSaved) < 10) return;
    lastProgressSaveRef.current[movieId] = roundedPosition;
    try {
      const response = await fetch(`/api/movies/${movieId}/progress`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ position: roundedPosition }),
      });
      if (!response.ok) {
        throw new Error(`Could not save progress (server returned ${response.status})`);
      }
      setWatchProgress((current) => {
        const updatedAt = new Date().toISOString();
        const existingIndex = current.findIndex((item) => item.movie_id === movieId);
        if (existingIndex === -1) {
          return [{ movie_id: movieId, user_id: 0, position: roundedPosition, updated_at: updatedAt }, ...current];
        }
        const next = [...current];
        next[existingIndex] = { ...next[existingIndex], position: roundedPosition, updated_at: updatedAt };
        return next;
      });
    } catch {
      // Ignore transient progress save failures to avoid interrupting playback.
    }
  };

  const toggleMovieInMyList = async (movieId: number) => {
    const inList = myList.some((movie) => movie.id === movieId);
    setMyListBusy(true);
    try {
      const response = await fetch(`/api/my-list/${movieId}`, {
        method: inList ? 'DELETE' : 'POST',
        credentials: 'include',
      });
      if (!response.ok) {
        throw new Error(`Could not update My List (server returned ${response.status})`);
      }
      await fetchMyList();
    } catch (err) {
      setLibraryError(err instanceof Error ? err.message : 'Could not update My List');
    } finally {
      setMyListBusy(false);
    }
  };

  const toggleFeaturedMyList = async () => {
    if (!featured) return;
    await toggleMovieInMyList(featured.id);
  };

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
      <LoginScreen onSubmit={handleLogin} loading={loading} error={error} accessURL={accessURL} accessAddress={accessAddress} />
    );
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="sticky top-0 z-40 border-b border-slate-800 bg-slate-950/80 backdrop-blur">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-3">
            <div className="rounded-xl bg-cyan-500/20 p-2 text-cyan-300"><Film size={22} /></div>
            <div>
              <p className="text-lg font-bold tracking-wide">FamilyFlix</p>
              <p className="text-xs text-slate-400">Private local Wi-Fi library</p>
            </div>
          </div>
          <nav className="hidden items-center gap-5 text-sm text-slate-300 md:flex" aria-label="Primary">
            <a
              href="#home"
              onClick={(e) => {
                e.preventDefault();
                scrollToSection('home');
              }}
              className="flex items-center gap-2 hover:text-white"
            >
              <Search size={16} />Home
            </a>
            <a
              href="#movies"
              onClick={(e) => {
                e.preventDefault();
                scrollToSection('movies');
              }}
              className="flex items-center gap-2 hover:text-white"
            >
              <FolderOpen size={16} />Movies
            </a>
            <a
              href="#continue-watching"
              onClick={(e) => {
                e.preventDefault();
                scrollToSection('continue-watching');
              }}
              className="flex items-center gap-2 hover:text-white"
            >
              <Clock3 size={16} />Continue Watching
            </a>
            <a
              href="#movies"
              onClick={(e) => {
                e.preventDefault();
                scrollToSection('movies');
              }}
              className="flex items-center gap-2 hover:text-white"
            >
              <Download size={16} />Downloads
            </a>
          </nav>
          <button onClick={handleLogout} className="rounded-full border border-slate-700 px-4 py-2 text-sm text-slate-200 hover:border-cyan-500 hover:text-white">
            Sign out
          </button>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-6 py-8">
        <section id="home" className="relative overflow-hidden rounded-3xl border border-slate-800 bg-gradient-to-r from-cyan-950/60 via-slate-900 to-slate-950 p-8 shadow-glow">
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
              <div className="mt-5 flex flex-wrap items-center gap-2 text-xs text-slate-300">
                <span className="rounded-full border border-slate-700 bg-slate-900/70 px-2.5 py-1.5">Private & secure</span>
                <span className="rounded-full border border-slate-700 bg-slate-900/70 px-2.5 py-1.5">LAN ready</span>
                <span className="rounded-full border border-slate-700 bg-slate-900/70 px-2.5 py-1.5">Family-first access</span>
              </div>
              <div className="mt-6 flex flex-wrap items-center gap-3">
                <button
                  type="button"
                  disabled={!featured}
                  onClick={() => featured && setPlayingMovie(featured)}
                  className="inline-flex items-center gap-2 rounded-full bg-cyan-500 px-5 py-3 font-semibold text-slate-950 hover:bg-cyan-400 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Play size={16} /> Watch now
                </button>
                <button
                  type="button"
                  disabled={!featured || myListBusy}
                  onClick={() => void toggleFeaturedMyList()}
                  className="rounded-full border border-slate-700 px-5 py-3 font-semibold text-white hover:border-cyan-500 hover:text-cyan-300 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {myListBusy ? 'Saving...' : featuredInMyList ? 'Saved to My List' : 'Watch later'}
                </button>
              </div>
            </div>
            <div className="rounded-2xl border border-slate-700 bg-slate-900/70 p-4 text-left backdrop-blur">
              <div className="mb-3 flex items-center justify-between gap-3">
                <p className="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-slate-400"><Server size={14} /> Local access</p>
                <div className="rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-1.5 text-cyan-300"><QrCode size={18} /></div>
              </div>
              <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
                <div className="mb-3 flex items-center justify-center rounded-lg bg-white p-2">
                  <QRCodeSVG value={accessURL} size={110} bgColor="#ffffff" fgColor="#0f172a" />
                </div>
                <p className="break-all font-mono text-lg font-bold text-white">{accessAddress}</p>
                <p className="mt-2 text-sm text-slate-300">Scan to open this exact address on your phone. Both devices must be on the same Wi‑Fi.</p>
              </div>
            </div>
          </div>
        </section>

        <section className="mt-10 grid gap-6 lg:grid-cols-3">
          <StatCard icon={<Tv size={18} />} title="Movies" value={String(movies.length)} subtitle="Available in the library" />
          <StatCard icon={<User size={18} />} title="Account" value={session.user?.username ?? 'Admin'} subtitle={session.user?.role ?? 'admin'} />
          <StatCard icon={<Download size={18} />} title="Network" value={network?.active_devices == null ? '—' : String(network.active_devices)} subtitle="Devices active in the last 2 minutes" />
        </section>

        {session.user?.role === 'admin' && (
          <section className="mt-10 rounded-2xl border border-slate-800 bg-slate-900/80 p-5" aria-labelledby="accounts-heading">
            <div className="mb-5">
              <p className="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-cyan-300"><Shield size={14} /> Admin only</p>
              <h2 id="accounts-heading" className="mt-2 text-2xl font-bold text-white">User accounts</h2>
              <p className="mt-1 text-sm text-slate-400">Create and remove normal user accounts. Only admins can manage accounts.</p>
            </div>
            <form
              className="grid gap-3 md:grid-cols-[1fr_1fr_auto] md:items-end"
              onSubmit={async (event) => {
                event.preventDefault();
                const form = event.currentTarget;
                const created = await createAccount(new FormData(form));
                if (created) form.reset();
              }}
            >
              <label className="block">
                <span className="mb-2 block text-sm text-slate-300">Username</span>
                <input name="username" required minLength={3} maxLength={32} pattern="[A-Za-z0-9._-]+" autoComplete="off" className="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2.5 text-white focus:border-cyan-500" />
              </label>
              <label className="block">
                <span className="mb-2 block text-sm text-slate-300">Temporary password</span>
                <input name="password" type="password" required minLength={8} maxLength={72} autoComplete="new-password" className="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2.5 text-white focus:border-cyan-500" />
              </label>
              <button type="submit" disabled={accountBusy} className="inline-flex items-center justify-center gap-2 rounded-xl bg-cyan-500 px-4 py-2.5 font-semibold text-slate-950 hover:bg-cyan-400 disabled:opacity-60">
                <UserPlus size={16} /> Create normal user
              </button>
            </form>
            {accountError && <p role="alert" className="mt-3 text-sm text-red-300">{accountError}</p>}
            {accountNotice && <p role="status" className="mt-3 text-sm text-emerald-300">{accountNotice}</p>}
            <div className="mt-5 divide-y divide-slate-800 rounded-xl border border-slate-800">
              {accounts.map((account) => (
                <div key={account.id} className="flex items-center justify-between gap-4 px-4 py-3">
                  <div>
                    <p className="font-medium text-white">{account.username}</p>
                    <p className="text-xs capitalize text-slate-400">{account.role}</p>
                  </div>
                  {account.role !== 'admin' && (
                    <button
                      type="button"
                      disabled={accountBusy}
                      onClick={() => void deleteAccount(account)}
                      className="inline-flex items-center gap-2 rounded-lg border border-red-500/30 px-3 py-2 text-sm text-red-300 hover:bg-red-500/10 disabled:opacity-60"
                      aria-label={`Delete ${account.username}`}
                    >
                      <Trash2 size={15} /> Delete
                    </button>
                  )}
                </div>
              ))}
              {accounts.length === 0 && <p className="px-4 py-3 text-sm text-slate-400">No accounts found.</p>}
            </div>
          </section>
        )}

        <section className="mt-10 rounded-2xl border border-slate-800 bg-slate-900/80 p-5">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300">Library overview</p>
              <h2 className="mt-2 text-2xl font-bold text-white">Professional home media control</h2>
            </div>
            <div className="inline-flex items-center gap-2 rounded-full border border-emerald-500/40 bg-emerald-500/10 px-3 py-1.5 text-sm text-emerald-300">
              <span className="h-2 w-2 rounded-full bg-emerald-400" />
              Operational & ready
            </div>
          </div>
          <div className="mt-5 grid gap-4 md:grid-cols-3">
            <InfoTile label="Streaming" value="HD & local playback" detail="Optimized for home viewing on shared devices" />
            <InfoTile label="Security" value="Session protected" detail="Cookie-based access with private admin control" />
            <InfoTile label="Library" value="Auto discovery" detail="Indexes files from the shared media folder" />
          </div>
        </section>

        <section id="continue-watching" className="mt-10">
          <div className="mb-6 flex items-center justify-between">
            <h2 className="text-2xl font-bold text-white">Continue Watching</h2>
            <span className="text-sm text-slate-400">{continueWatchingMovies.length} in progress</span>
          </div>
          {continueWatchingMovies.length === 0 ? (
            <p className="text-sm text-slate-400">Movies you start watching will appear here so you can resume them later.</p>
          ) : (
            <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-4">
              {continueWatchingMovies.map((movie) => {
                const position = getSavedPosition(movie.id);
                return (
                  <div key={`continue-${movie.id}`} className="group overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/80 transition hover:-translate-y-1 hover:border-cyan-500/50 hover:shadow-lg hover:shadow-cyan-500/10">
                    <div className="aspect-[3/4] bg-gradient-to-br from-slate-700 to-slate-900 p-3">
                      <div className="flex h-full items-end justify-between rounded-xl border border-slate-700 bg-[radial-gradient(circle_at_top,rgba(34,211,238,0.22),transparent_35%),linear-gradient(135deg,#0f172a,#111827)] p-4">
                        <span className="rounded-full bg-slate-950/75 px-2 py-1 text-xs uppercase tracking-wide text-cyan-300">{movie.format}</span>
                        <span className="rounded-full bg-slate-950/75 px-2 py-1 text-xs text-slate-200">{Math.floor(position / 60)}m watched</span>
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
                          aria-label={`Resume ${movie.title}`}
                        >
                          <Play size={15} />
                        </button>
                      </div>
                      <p className="line-clamp-3 text-sm text-slate-400">{movie.description || 'No description yet.'}</p>
                      <div className="flex items-center justify-between gap-3 text-xs text-slate-400">
                        <span>Resume from {Math.floor(position / 60)}:{String(Math.floor(position % 60)).padStart(2, '0')}</span>
                        <div className="flex items-center gap-3">
                          <button
                            type="button"
                            onClick={() => void toggleMovieInMyList(movie.id)}
                            disabled={myListBusy}
                            className="font-medium text-cyan-300 hover:text-cyan-200 disabled:opacity-60"
                            aria-label={isMovieInMyList(movie.id) ? `Remove ${movie.title} from My List` : `Save ${movie.title} for watch later`}
                          >
                            {isMovieInMyList(movie.id) ? 'Saved' : 'Watch later'}
                          </button>
                          <a href={`/api/movies/${movie.id}/download`} className="font-medium text-cyan-300 hover:text-cyan-200">Download</a>
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </section>

        <section id="my-list" className="mt-10">
          <div className="mb-6 flex items-center justify-between">
            <h2 className="text-2xl font-bold text-white">My List</h2>
            <span className="text-sm text-slate-400">{myList.length} saved</span>
          </div>
          {myList.length === 0 ? (
            <p className="text-sm text-slate-400">Save movies here to watch them later.</p>
          ) : (
            <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-4">
              {myList.map((movie) => (
                <div key={`my-list-${movie.id}`} className="group overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/80 transition hover:-translate-y-1 hover:border-cyan-500/50 hover:shadow-lg hover:shadow-cyan-500/10">
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
                      <button
                        type="button"
                        onClick={async () => {
                          setMyListBusy(true);
                          try {
                            const response = await fetch(`/api/my-list/${movie.id}`, { method: 'DELETE', credentials: 'include' });
                            if (!response.ok) {
                              throw new Error(`Could not update My List (server returned ${response.status})`);
                            }
                            await fetchMyList();
                          } catch (err) {
                            setLibraryError(err instanceof Error ? err.message : 'Could not update My List');
                          } finally {
                            setMyListBusy(false);
                          }
                        }}
                        disabled={myListBusy}
                        className="font-medium text-cyan-300 hover:text-cyan-200 disabled:opacity-60"
                      >
                        Remove
                      </button>
                      <a href={`/api/movies/${movie.id}/download`} className="font-medium text-cyan-300 hover:text-cyan-200">Download</a>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        <section id="movies" className="mt-10">
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
                  <div className="flex items-center justify-between gap-3 text-xs text-slate-400">
                    <button
                      type="button"
                      onClick={() => void toggleMovieInMyList(movie.id)}
                      disabled={myListBusy}
                      className="font-medium text-cyan-300 hover:text-cyan-200 disabled:opacity-60"
                      aria-label={isMovieInMyList(movie.id) ? `Remove ${movie.title} from My List` : `Save ${movie.title} for watch later`}
                    >
                      {isMovieInMyList(movie.id) ? 'Saved' : 'Watch later'}
                    </button>
                    <a href={`/api/movies/${movie.id}/download`} className="font-medium text-cyan-300 hover:text-cyan-200">Download</a>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </section>
      </main>
      {playingMovie && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 p-4" role="dialog" aria-modal="true" aria-label="Movie player">
          <div className="w-full max-w-5xl overflow-hidden rounded-2xl border border-slate-700 bg-slate-900 shadow-2xl">
            <div className="flex items-center justify-between gap-4 p-4">
              <h2 className="truncate text-lg font-semibold text-white">{playingMovie.title}</h2>
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
              onLoadedMetadata={(e) => {
                const savedPosition = getSavedPosition(playingMovie.id);
                if (savedPosition > 0) {
                  e.currentTarget.currentTime = savedPosition;
                }
              }}
              onTimeUpdate={(e) => {
                void saveMovieProgress(playingMovie.id, e.currentTarget.currentTime);
              }}
              onPause={(e) => {
                void saveMovieProgress(playingMovie.id, e.currentTarget.currentTime, true);
              }}
              onEnded={() => {
                void saveMovieProgress(playingMovie.id, 0, true);
              }}
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

function LoginScreen({ onSubmit, loading, error, accessURL, accessAddress }: { onSubmit: (formData: FormData) => void; loading: boolean; error: string | null; accessURL: string; accessAddress: string;}) {
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
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p className="text-xs uppercase tracking-[0.18em] text-slate-400">Open on your phone</p>
                  <p className="mt-2 break-all font-mono text-lg text-white">{accessAddress}</p>
                  <p className="mt-2 text-sm text-slate-300">Scan this code or enter the address on a device connected to the same Wi‑Fi.</p>
                </div>
                <div className="flex items-center justify-center rounded-2xl border border-slate-700 bg-white p-2 shadow-lg shadow-cyan-950/20">
                  <QRCodeSVG value={accessURL} size={110} bgColor="#ffffff" fgColor="#0f172a" />
                </div>
              </div>
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
              <input name="username" autoComplete="username" className="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-3 text-white outline-none ring-0 placeholder:text-slate-500 focus:border-cyan-500" placeholder="Username" />
            </label>

            <label className="block">
              <span className="mb-2 block text-sm text-slate-300">Password</span>
              <input type="password" name="password" autoComplete="current-password" className="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-3 text-white placeholder:text-slate-500 focus:border-cyan-500" placeholder="Password" />
            </label>

            {error && <p className="rounded-xl border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">{error}</p>}

            <button type="submit" disabled={loading} className="w-full rounded-xl bg-cyan-500 px-4 py-3 font-semibold text-slate-950 transition hover:bg-cyan-400 disabled:cursor-not-allowed disabled:opacity-70">
              {loading ? 'Signing in...' : 'Sign in'}
            </button>

            <p className="border-t border-slate-800 pt-4 text-xs text-slate-400">Sign in with the account provided by your FamilyFix administrator.</p>
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

function InfoTile({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-950/40 p-4">
      <p className="text-xs uppercase tracking-[0.18em] text-slate-400">{label}</p>
      <p className="mt-3 text-xl font-semibold text-white">{value}</p>
      <p className="mt-2 text-sm text-slate-400">{detail}</p>
    </div>
  );
}

export default App;
