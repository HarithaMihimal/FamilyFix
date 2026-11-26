import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import App from './App';

describe('FamilyFlix app', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: false,
        status: 401,
        json: async () => ({ authenticated: false }),
      })),
    );
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('renders the sign in screen by default', async () => {
    render(<App />);
    expect(await screen.findByText(/Sign in to FamilyFlix/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign up/i })).toBeInTheDocument();
    expect(screen.getByText('192.168.8.173:5173')).toBeInTheDocument();
  });

  it('submits signups for admin approval', async () => {
    const fetchMock = vi.fn(async (url: string) => {
      if (url === '/api/session') {
        return { ok: false, status: 401, json: async () => ({ authenticated: false }) };
      }
      if (url === '/api/signup') {
        return { ok: true, status: 202, json: async () => ({ status: 'pending_approval' }) };
      }
      return { ok: true, json: async () => ({ local_server: 'http://localhost:3000' }) };
    });
    vi.stubGlobal('fetch', fetchMock);
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /New here\? Sign up/i }));
    expect(screen.getByRole('heading', { name: 'Create your account' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'new-family-user' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'new-family-pass-123' } });
    fireEvent.click(screen.getByRole('button', { name: 'Request account' }));

    expect(await screen.findByRole('status')).toHaveTextContent(/must approve it before you can sign in/i);
    expect(fetchMock).toHaveBeenCalledWith('/api/signup', expect.objectContaining({ method: 'POST' }));
  });

  it('shows a retry option when the backend cannot be reached', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Failed to fetch')));
    render(<App />);

    expect(await screen.findByText(/Can’t connect to FamilyFlix/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('plays a movie from the streaming endpoint and keeps its download link', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url === '/api/session') {
        return {
          ok: true,
          json: async () => ({ authenticated: true, user: { username: 'admin', role: 'admin' } }),
        };
      }
      if (url === '/api/movies') {
        return {
          ok: true,
          json: async () => [{
            id: 7,
            title: 'Sample Movie',
            file_name: 'sample.mp4',
            file_path: 'sample.mp4',
            description: 'A sample movie',
            genre: 'Drama',
            format: 'MP4',
            file_size: 1024,
          }],
        };
      }
      if (url === '/api/my-list') {
        return { ok: true, json: async () => [] };
      }
      return { ok: true, json: async () => ({ local_server: 'http://localhost:3000', active_devices: 3 }) };
    }));

    render(<App />);
    fireEvent.click(await screen.findByRole('button', { name: 'Play Sample Movie' }));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByLabelText('Sample Movie')).toHaveAttribute('src', '/api/movies/7/stream');
    expect(screen.getByRole('link', { name: 'Download instead' })).toHaveAttribute('href', '/api/movies/7/download');
  });

  it('adds and removes the featured movie from My List', async () => {
    let myList: Array<{ id: number; title: string; file_name: string; file_path: string; description: string; genre: string; format: string; file_size: number }> = [];
    const movie = {
      id: 7,
      title: 'Sample Movie',
      file_name: 'sample.mp4',
      file_path: 'sample.mp4',
      description: 'A sample movie',
      genre: 'Drama',
      format: 'MP4',
      file_size: 1024,
    };

    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url === '/api/session') {
        return { ok: true, json: async () => ({ authenticated: true, user: { username: 'admin', role: 'admin' } }) };
      }
      if (url === '/api/movies') {
        return { ok: true, json: async () => [movie] };
      }
      if (url === '/api/my-list' && (!init || !init.method || init.method === 'GET')) {
        return { ok: true, json: async () => myList };
      }
      if (url === '/api/my-list/7' && init?.method === 'POST') {
        myList = [movie];
        return { ok: true, status: 204, json: async () => ({}) };
      }
      if (url === '/api/my-list/7' && init?.method === 'DELETE') {
        myList = [];
        return { ok: true, status: 204, json: async () => ({}) };
      }
      return { ok: true, json: async () => ({ local_server: 'http://localhost:3000', active_devices: 1 }) };
    }));

    render(<App />);

    const addButton = await screen.findByRole('button', { name: /^Watch later$/i });
    fireEvent.click(addButton);

    expect(await screen.findByRole('button', { name: /Saved to My List/i })).toBeInTheDocument();
    const myListSection = document.getElementById('my-list');
    expect(myListSection).not.toBeNull();
    expect(within(myListSection as HTMLElement).getByRole('heading', { name: 'My List' })).toBeInTheDocument();
    expect(within(myListSection as HTMLElement).getByText('Sample Movie')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Saved to My List/i }));
    expect(await screen.findByRole('button', { name: /^Watch later$/i })).toBeInTheDocument();
    expect(within(myListSection as HTMLElement).getByText(/Save movies here to watch them later/i)).toBeInTheDocument();
  });

  it('lets admins create normal user accounts', async () => {
    let users = [{ id: 1, username: 'admin', role: 'admin', approved: true, created_at: '2026-01-01' }];
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url === '/api/session') {
        return { ok: true, json: async () => ({ authenticated: true, user: { username: 'admin', role: 'admin' } }) };
      }
      if (url === '/api/admin/users' && (!init?.method || init.method === 'GET')) {
        return { ok: true, json: async () => users };
      }
      if (url === '/api/admin/users' && init?.method === 'POST') {
        users = [...users, { id: 2, username: 'family-user', role: 'user', approved: true, created_at: '2026-01-02' }];
        return { ok: true, status: 201, json: async () => users[1] };
      }
      return { ok: true, json: async () => [] };
    }));

    render(<App />);
    expect(await screen.findByRole('heading', { name: 'User accounts' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'family-user' } });
    fireEvent.change(screen.getByLabelText('Temporary password'), { target: { value: 'family-pass-123' } });
    fireEvent.click(screen.getByRole('button', { name: /Create normal user/i }));

    expect(await screen.findByText('Normal user account created.')).toBeInTheDocument();
    expect(screen.getByText('family-user')).toBeInTheDocument();
    expect(screen.getByText('user')).toBeInTheDocument();
  });

  it('does not show account management to normal users', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url === '/api/session') {
        return { ok: true, json: async () => ({ authenticated: true, user: { username: 'family-user', role: 'user' } }) };
      }
      return { ok: true, json: async () => [] };
    }));

    render(<App />);
    expect(await screen.findByText('family-user')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'User accounts' })).not.toBeInTheDocument();
    expect(document.querySelector('.user-theme')).not.toBeNull();
  });

  it('lets admins approve pending users', async () => {
    let users = [
      { id: 1, username: 'admin', role: 'admin', approved: true, created_at: '2026-01-01' },
      { id: 2, username: 'pending-user', role: 'user', approved: false, created_at: '2026-01-02' },
    ];
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url === '/api/session') {
        return { ok: true, json: async () => ({ authenticated: true, user: { username: 'admin', role: 'admin' } }) };
      }
      if (url === '/api/admin/users' && !init?.method) {
        return { ok: true, json: async () => users };
      }
      if (url === '/api/admin/users/2/approve' && init?.method === 'POST') {
        users = users.map((user) => user.id === 2 ? { ...user, approved: true } : user);
        return { ok: true, status: 204, json: async () => ({}) };
      }
      return { ok: true, json: async () => [] };
    }));

    render(<App />);
    expect(await screen.findByText('Pending approval')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Approve pending-user' }));
    expect(await screen.findByText('Account "pending-user" approved.')).toBeInTheDocument();
    expect(screen.getByText('user')).toBeInTheDocument();
  });
});
