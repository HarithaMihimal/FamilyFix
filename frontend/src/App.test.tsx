import { fireEvent, render, screen } from '@testing-library/react';
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
    vi.unstubAllGlobals();
  });

  it('renders the sign in screen by default', async () => {
    render(<App />);
    expect(await screen.findByText(/Sign in to FamilyFlix/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
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
      return { ok: true, json: async () => ({ local_server: 'http://localhost:3000' }) };
    }));

    render(<App />);
    fireEvent.click(await screen.findByRole('button', { name: 'Play Sample Movie' }));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.getByLabelText('Sample Movie')).toHaveAttribute('src', '/api/movies/7/stream');
    expect(screen.getByRole('link', { name: 'Download instead' })).toHaveAttribute('href', '/api/movies/7/download');
  });
});
