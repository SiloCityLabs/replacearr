import React, { useEffect, useState } from 'react';
import {
  ArrowLeft,
  CheckCircle2,
  Clock,
  Film,
  Folder,
  Loader2,
  Magnet,
  Pencil,
  RefreshCw,
  Search,
  Trash2,
  UserRound,
} from 'lucide-react';
import type {
  Movie,
  MovieExtraFile,
  MovieFilesResponse,
  MoviePersonCredit,
  QualityProfile,
  ReleaseSearchRequest,
  Tag,
} from '../types';
import { api, formatBytes } from '../api';
import type { ManualTorrentTarget } from './ManualTorrentForm';
import { MovieEditModal } from './modals/MovieEditModal';
import { SubtitlesPanel } from './SubtitlesPanel';

interface MovieDetailPanelProps {
  movie: Movie;
  posterUrl: string | null;
  profiles: QualityProfile[];
  tags: Tag[];
  onBack: () => void;
  onDelete: () => void;
  onSearchReleases: (req: ReleaseSearchRequest) => void;
  onManualTorrent: (target: ManualTorrentTarget) => void;
  onNotify?: (msg: string, isError?: boolean) => void;
  onUpdated?: (movie: Movie) => void;
}

const LANG_NAMES: Record<string, string> = {
  en: 'English',
  es: 'Spanish',
  fr: 'French',
  de: 'German',
  it: 'Italian',
  pt: 'Portuguese',
  ru: 'Russian',
  ja: 'Japanese',
  zh: 'Chinese',
  ko: 'Korean',
  hi: 'Hindi',
  ar: 'Arabic',
};

function languageLabel(code: string): string {
  const c = (code || '').toLowerCase();
  if (!c) return '—';
  return LANG_NAMES[c] || code.toUpperCase();
}

function formatRating(value?: number): string {
  if (value == null || Number.isNaN(value)) return '—';
  return Number.isInteger(value) ? String(value) : value.toFixed(1);
}

function personImage(p: MoviePersonCredit): string | null {
  const img = p.images?.find((i) => i.coverType === 'headshot') || p.images?.[0];
  return img?.url || img?.remoteUrl || null;
}

function formatDate(v?: string): string {
  if (!v) return '—';
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return v.slice(0, 10);
  return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export const MovieDetailPanel: React.FC<MovieDetailPanelProps> = ({
  movie: initial,
  posterUrl,
  profiles,
  tags,
  onBack,
  onDelete,
  onSearchReleases,
  onManualTorrent,
  onNotify,
  onUpdated,
}) => {
  const [movie, setMovie] = useState<Movie>(initial);
  const [files, setFiles] = useState<MovieFilesResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [editOpen, setEditOpen] = useState(false);

  const load = async (id: number, refreshMeta: boolean) => {
    setLoading(true);
    try {
      let next = await api.getMovie(id);
      if (refreshMeta && next.tmdbId > 0 && !(next.credits?.cast?.length || next.ratings?.imdb || next.ratings?.tmdb)) {
        try {
          next = await api.refreshMovie(id);
        } catch {
          // keep DB copy if metadata refresh fails
        }
      }
      setMovie(next);
      onUpdated?.(next);
      const filePayload = await api.getMovieFiles(id);
      setFiles(filePayload);
    } catch (err) {
      onNotify?.(err instanceof Error ? err.message : 'Failed to load movie', true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load(initial.id, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial.id]);

  const searchMovie = () => {
    onSearchReleases({
      query: `${movie.title} ${movie.year || ''}`.trim(),
      type: 'movie',
      movieId: movie.id,
      tmdbId: movie.tmdbId || undefined,
      contextLabel: movie.title,
    });
  };

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      const updated = await api.refreshMovie(movie.id);
      setMovie(updated);
      onUpdated?.(updated);
      const filePayload = await api.getMovieFiles(movie.id);
      setFiles(filePayload);
      onNotify?.('Metadata refreshed');
    } catch (err) {
      onNotify?.(err instanceof Error ? err.message : 'Metadata refresh failed', true);
    } finally {
      setRefreshing(false);
    }
  };

  const cast = movie.credits?.cast || [];
  const crew = (movie.credits?.crew || []).filter((c) =>
    ['Director', 'Screenplay', 'Writer', 'Producer', 'Director of Photography'].includes(c.job || ''),
  );
  const alts = movie.alternateTitles || [];
  const mediaFiles = files?.media || [];
  const extraFiles = files?.extra || [];
  const profileName =
    profiles.find((p) => p.id === movie.qualityProfileId)?.name || `Profile #${movie.qualityProfileId || '—'}`;
  const tagLabels = (movie.tags || [])
    .map((id) => tags.find((t) => t.id === id)?.label)
    .filter((v): v is string => Boolean(v));
  const genresFull = (movie.genres || []).join(', ') || '—';
  const genresDisplay =
    genresFull.length > 20 ? `${genresFull.slice(0, 20)}...` : genresFull;

  return (
    <div className="min-h-[70vh] flex flex-col bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
      <div className="p-4 sm:p-5 border-b border-slate-800 flex flex-wrap items-center justify-between gap-3 shrink-0">
        <button
          onClick={onBack}
          className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-800 text-slate-200 hover:text-white text-xs font-semibold transition"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to library
        </button>
        <div className="flex flex-wrap items-center gap-2">
          <button
            onClick={() => setEditOpen(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-sky-500/15 text-sky-300 border border-sky-500/30 text-xs font-semibold transition"
          >
            <Pencil className="w-3.5 h-3.5" />
            Edit
          </button>
          <button
            onClick={() => void handleRefresh()}
            disabled={refreshing}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 text-slate-200 border border-slate-700 text-xs font-semibold transition disabled:opacity-50"
          >
            {refreshing ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
            Refresh metadata
          </button>
          <button
            onClick={() =>
              onManualTorrent({
                title: movie.title,
                mediaType: 'movie',
                movieId: movie.id,
              })
            }
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500/15 text-amber-300 border border-amber-500/30 text-xs font-semibold transition"
          >
            <Magnet className="w-3.5 h-3.5" />
            Add torrent
          </button>
          <button
            onClick={searchMovie}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold transition"
          >
            <Search className="w-3.5 h-3.5" />
            Interactive Search
          </button>
          <button
            onClick={onDelete}
            className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
            title="Delete movie"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>

      <div className="p-5 flex flex-col gap-6 overflow-y-auto">
        <div className="flex flex-col sm:flex-row gap-5">
          <div className="w-48 h-72 mx-auto sm:mx-0 rounded-lg bg-gradient-to-br from-emerald-900/60 via-slate-900 to-slate-950 border border-slate-800 shrink-0 overflow-hidden flex items-center justify-center">
            {posterUrl ? (
              <img src={posterUrl} alt={movie.title} className="w-full h-full object-cover" />
            ) : (
              <Film className="w-10 h-10 text-emerald-400/60" />
            )}
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2 mb-2">
              {movie.tmdbId > 0 && (
                <span className="text-xs font-mono uppercase px-2 py-0.5 rounded bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                  TMDB #{movie.tmdbId}
                </span>
              )}
              {movie.imdbId && (
                <span className="text-xs font-mono uppercase px-2 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700">
                  {movie.imdbId}
                </span>
              )}
              {movie.monitored ? (
                <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                  Monitored
                </span>
              ) : (
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 border border-slate-700">
                  Unmonitored
                </span>
              )}
              {movie.hasFile ? (
                <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                  <CheckCircle2 className="w-3 h-3" /> Downloaded
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-300 border border-amber-500/30">
                  <Clock className="w-3 h-3" /> Missing
                </span>
              )}
              {movie.year > 0 && (
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                  {movie.year}
                </span>
              )}
              {movie.certification && (
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                  {movie.certification}
                </span>
              )}
              {movie.runtime > 0 && (
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                  {movie.runtime} min
                </span>
              )}
              {movie.studio && (
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-emerald-300 border border-slate-700">
                  {movie.studio}
                </span>
              )}
              {movie.sizeOnDisk > 0 && (
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                  {formatBytes(movie.sizeOnDisk)}
                </span>
              )}
            </div>
            <h2 className="text-2xl font-bold text-white">{movie.title}</h2>
            {movie.originalTitle && movie.originalTitle !== movie.title && (
              <p className="text-sm text-slate-400 mt-1">{movie.originalTitle}</p>
            )}
            <div className="mt-2.5 flex flex-wrap items-center gap-x-5 gap-y-2">
              <RatingChip kind="tmdb" value={movie.ratings?.tmdb?.value} votes={movie.ratings?.tmdb?.votes} />
              <RatingChip kind="imdb" value={movie.ratings?.imdb?.value} votes={movie.ratings?.imdb?.votes} />
              <RatingChip kind="rt" value={movie.ratings?.rottenTomatoes?.value} />
              <RatingChip kind="metacritic" value={movie.ratings?.metacritic?.value} />
            </div>
            <p className="mt-3 text-sm text-slate-300 leading-relaxed">
              {movie.overview || 'No overview available.'}
            </p>

            <div className="mt-4 flex flex-wrap gap-x-6 gap-y-3 text-sm">
              <div className="min-w-0 shrink-0 max-w-full">
                <div className="text-[11px] uppercase tracking-wide text-slate-500">Path</div>
                <div className="mt-0.5 flex items-center gap-1.5 text-xs font-mono text-slate-400">
                  <Folder className="w-3.5 h-3.5 shrink-0" />
                  <span
                    className="truncate max-w-[18rem]"
                    title={movie.path || `${movie.rootFolderPath}/${movie.title}`}
                  >
                    {movie.path || `${movie.rootFolderPath}/${movie.title}`}
                  </span>
                </div>
              </div>
              <MetaRow
                label="Genres"
                value={genresDisplay}
                title={genresFull.length > 20 ? genresFull : undefined}
              />
              <MetaRow label="Original language" value={languageLabel(movie.originalLanguage)} />
              <MetaRow label="Collection" value={movie.collection?.name || '—'} />
              <MetaRow label="In cinemas" value={formatDate(movie.inCinemas)} />
              <MetaRow label="Digital" value={formatDate(movie.digitalRelease)} />
              <MetaRow label="Physical" value={formatDate(movie.physicalRelease)} />
              <MetaRow label="Quality profile" value={profileName} />
              <MetaRow
                label="Minimum availability"
                value={movie.minimumAvailability || 'released'}
              />
              <MetaRow label="Tags" value={tagLabels.length ? tagLabels.join(', ') : '—'} />
            </div>
          </div>
        </div>

        <SubtitlesPanel kind="movie" id={movie.id} hasFile={movie.hasFile} onNotify={onNotify} />

        <section className="rounded-xl border border-slate-800 bg-slate-950/50 p-4">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-400 mb-3">Media files</h3>
          {loading && !files ? (
            <p className="text-xs text-slate-500">Loading files…</p>
          ) : mediaFiles.length === 0 ? (
            <p className="text-xs text-slate-500">No movie file on disk</p>
          ) : (
            <FileTable
              headers={['Relative path', 'Quality', 'Resolution', 'Codecs', 'Size', 'Release group', 'Added']}
              rows={mediaFiles.map((f) => [
                f.relativePath || f.path,
                f.quality || '—',
                f.resolution && f.resolution > 0 ? `${f.resolution}p` : '—',
                [f.videoCodec, f.audioCodec].filter(Boolean).join(' / ') || '—',
                formatBytes(f.size),
                f.releaseGroup || '—',
                formatDate(f.dateAdded),
              ])}
            />
          )}
        </section>

        {extraFiles.length > 0 && (
          <section className="rounded-xl border border-slate-800 bg-slate-950/50 p-4">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-400 mb-3">Other files</h3>
            <FileTable
              headers={['Name', 'Type', 'Ext', 'Size']}
              rows={extraFiles.map((f: MovieExtraFile) => [
                f.relativePath,
                f.type,
                f.extension || '—',
                formatBytes(f.size),
              ])}
            />
          </section>
        )}

        {(cast.length > 0 || crew.length > 0) && (
          <section className="rounded-xl border border-slate-800 bg-slate-950/50 p-4 space-y-5">
            {cast.length > 0 && (
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-400 mb-3">Cast</h3>
                <div className="flex gap-3 overflow-x-auto pb-1 -mx-1 px-1 scroll-smooth">
                  {cast.map((person) => (
                    <PersonCard key={`cast-${person.tmdbId}-${person.order}`} person={person} subtitle={person.character} />
                  ))}
                </div>
              </div>
            )}
            {crew.length > 0 && (
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-400 mb-3">Crew</h3>
                <div className="flex gap-3 overflow-x-auto pb-1 -mx-1 px-1 scroll-smooth">
                  {crew.map((person) => (
                    <PersonCard
                      key={`crew-${person.tmdbId}-${person.job}-${person.order}`}
                      person={person}
                      subtitle={person.job}
                    />
                  ))}
                </div>
              </div>
            )}
          </section>
        )}

        <section className="rounded-xl border border-slate-800 bg-slate-950/50 p-4">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-400 mb-3">Alternate titles</h3>
          {alts.length === 0 ? (
            <p className="text-xs text-slate-500">No alternate titles</p>
          ) : (
            <div className="max-h-64 overflow-y-auto">
              <FileTable
                headers={['Title', 'Language', 'Source']}
                rows={alts.map((a) => [a.title, a.language || '—', a.type || '—'])}
              />
            </div>
          )}
        </section>
      </div>

      {editOpen && (
        <MovieEditModal
          movie={movie}
          profiles={profiles}
          tags={tags}
          onClose={() => setEditOpen(false)}
          onSaved={(updated) => {
            setMovie(updated);
            onUpdated?.(updated);
          }}
          onNotify={onNotify}
        />
      )}
    </div>
  );
};

const MetaRow: React.FC<{ label: string; value: string; title?: string }> = ({
  label,
  value,
  title,
}) => (
  <div className="min-w-0 shrink-0">
    <div className="text-[11px] uppercase tracking-wide text-slate-500">{label}</div>
    <div className="text-slate-200 mt-0.5 whitespace-nowrap" title={title}>
      {value}
    </div>
  </div>
);

type RatingKind = 'tmdb' | 'imdb' | 'rt' | 'metacritic';

function formatRatingDisplay(kind: RatingKind, value: number): string {
  if (kind === 'tmdb') {
    const pct = value <= 10 ? Math.round(value * 10) : Math.round(value);
    return `${pct}%`;
  }
  if (kind === 'rt') {
    return `${Math.round(value)}%`;
  }
  if (kind === 'metacritic') {
    return String(Math.round(value));
  }
  return formatRating(value);
}

const RatingIcon: React.FC<{ kind: RatingKind }> = ({ kind }) => {
  if (kind === 'tmdb') {
    return (
      <svg viewBox="0 0 36 16" className="h-3.5 w-auto" aria-hidden>
        <defs>
          <linearGradient id="ra-tmdb-grad" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor="#01b4e4" />
            <stop offset="100%" stopColor="#90cea1" />
          </linearGradient>
        </defs>
        <text x="0" y="7.5" fill="#01b4e4" fontSize="6.5" fontWeight="800" fontFamily="system-ui,sans-serif">
          TM
        </text>
        <rect x="11" y="1.5" width="14" height="5" rx="2.5" fill="#01b4e4" />
        <rect x="8" y="9" width="17" height="5" rx="2.5" fill="url(#ra-tmdb-grad)" />
        <text x="26.5" y="14" fill="#01b4e4" fontSize="6.5" fontWeight="800" fontFamily="system-ui,sans-serif">
          DB
        </text>
      </svg>
    );
  }
  if (kind === 'imdb') {
    return (
      <span className="inline-flex items-center justify-center h-3.5 px-1 rounded-[3px] bg-[#f5c518] text-[9px] font-black leading-none text-black tracking-tight">
        IMDb
      </span>
    );
  }
  if (kind === 'rt') {
    return (
      <svg viewBox="0 0 20 20" className="h-3.5 w-3.5" aria-hidden>
        <circle cx="10" cy="11.5" r="6.5" fill="#fa320a" />
        <path
          d="M8.2 4.8c.9-1.3 1.9-2 2.8-2.2.2.9-.1 1.8-.5 2.5 1.1-.3 2.1-.2 3 .3-.6.9-1.6 1.4-2.7 1.5-.3 1-.1 1.7-2 2-.2-1.2.1-2.4.7-3.4-.6.2-1.1.6-1.3 1z"
          fill="#2ea043"
        />
      </svg>
    );
  }
  return (
    <span className="inline-flex items-center justify-center h-3.5 min-w-3.5 px-0.5 rounded-[2px] bg-[#ffcc33] text-[9px] font-black leading-none text-black">
      M
    </span>
  );
};

const RatingChip: React.FC<{ kind: RatingKind; value?: number; votes?: number }> = ({
  kind,
  value,
  votes,
}) => {
  if (value == null) return null;
  const label =
    kind === 'tmdb'
      ? 'TMDB'
      : kind === 'imdb'
        ? 'IMDb'
        : kind === 'rt'
          ? 'Rotten Tomatoes'
          : 'Metacritic';
  const tip =
    votes != null && votes > 0 ? `${label}: ${formatRatingDisplay(kind, value)} (${votes.toLocaleString()} votes)` : label;
  return (
    <span className="inline-flex items-center gap-1.5 text-sm text-slate-300" title={tip}>
      <RatingIcon kind={kind} />
      <span className="tabular-nums">{formatRatingDisplay(kind, value)}</span>
    </span>
  );
};

const PersonCard: React.FC<{ person: MoviePersonCredit; subtitle?: string }> = ({ person, subtitle }) => {
  const src = personImage(person);
  const href = person.tmdbId > 0 ? `https://www.themoviedb.org/person/${person.tmdbId}` : undefined;
  const className =
    'w-28 shrink-0 rounded-lg border border-slate-800 bg-slate-900/60 overflow-hidden block transition hover:border-sky-500/40 hover:bg-slate-900';
  const body = (
    <>
      <div className="aspect-[2/3] bg-slate-950 flex items-center justify-center">
        {src ? (
          <img src={src} alt={person.name} className="w-full h-full object-cover" loading="lazy" />
        ) : (
          <UserRound className="w-8 h-8 text-slate-600" />
        )}
      </div>
      <div className="p-2">
        <div className="text-xs font-medium text-white truncate" title={person.name}>
          {person.name}
        </div>
        {subtitle && (
          <div className="text-[11px] text-slate-400 truncate" title={subtitle}>
            {subtitle}
          </div>
        )}
      </div>
    </>
  );
  if (!href) {
    return <div className={className}>{body}</div>;
  }
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={className}
      title={`Open ${person.name} on TMDB`}
    >
      {body}
    </a>
  );
};

const FileTable: React.FC<{ headers: string[]; rows: string[][] }> = ({ headers, rows }) => (
  <div className="overflow-x-auto">
    <table className="w-full text-left text-xs">
      <thead>
        <tr className="text-slate-500 border-b border-slate-800">
          {headers.map((h) => (
            <th key={h} className="py-2 pr-3 font-medium">
              {h}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={i} className="border-b border-slate-800/60 text-slate-300">
            {row.map((cell, j) => (
              <td key={j} className={`py-2 pr-3 ${j === 0 ? 'font-mono break-all' : ''}`}>
                {cell}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  </div>
);
