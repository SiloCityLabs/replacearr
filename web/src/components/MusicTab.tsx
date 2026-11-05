import React, { useMemo, useState } from 'react';
import { Music, Plus, Search, X, Disc3 } from 'lucide-react';
import type { Album, Artist, QualityProfile, ReleaseSearchRequest, RootFolder } from '../types';
import { api } from '../api';
import { useConfirm } from './modals/ConfirmModal';

interface MusicTabProps {
  artists: Artist[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onRefresh: () => Promise<void>;
  onSearchReleases: (req: ReleaseSearchRequest) => void;
  onNotify: (msg: string, isError?: boolean) => void;
  openAddPanel?: boolean;
  openAddNonce?: number;
  onAddPanelOpened?: () => void;
}

function pickMusicProfiles(profiles: QualityProfile[]): QualityProfile[] {
  const music = profiles.filter((p) => p.mediaType === 'music');
  return music.length > 0 ? music : profiles;
}

export const MusicTab: React.FC<MusicTabProps> = ({
  artists,
  profiles,
  rootFolders,
  onRefresh,
  onSearchReleases,
  onNotify,
  openAddPanel = false,
  openAddNonce = 0,
  onAddPanelOpened,
}) => {
  const { confirm, dialog } = useConfirm();
  const musicRoots = useMemo(
    () => rootFolders.filter((r) => r.mediaType === 'music' || r.mediaType === 'all'),
    [rootFolders]
  );
  const musicProfiles = useMemo(() => pickMusicProfiles(profiles), [profiles]);

  const [filterText, setFilterText] = useState('');
  const [showAddPanel, setShowAddPanel] = useState(false);
  const [lookupTerm, setLookupTerm] = useState('');
  const [lookupResults, setLookupResults] = useState<Artist[]>([]);
  const [lookupLoading, setLookupLoading] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState<number>(0);
  const [selectedRootFolder, setSelectedRootFolder] = useState('');
  const [activeArtist, setActiveArtist] = useState<Artist | null>(null);
  const [albums, setAlbums] = useState<Album[]>([]);

  React.useEffect(() => {
    if (musicProfiles.length && (!selectedProfileId || !musicProfiles.some((p) => p.id === selectedProfileId))) {
      setSelectedProfileId(musicProfiles[0].id);
    }
  }, [musicProfiles, selectedProfileId]);

  React.useEffect(() => {
    if (musicRoots.length && (!selectedRootFolder || !musicRoots.some((r) => r.path === selectedRootFolder))) {
      setSelectedRootFolder(musicRoots[0].path);
    }
    if (!musicRoots.length) {
      setSelectedRootFolder('');
    }
  }, [musicRoots, selectedRootFolder]);

  React.useEffect(() => {
    if (openAddPanel) {
      setShowAddPanel(true);
      onAddPanelOpened?.();
    }
  }, [openAddPanel, openAddNonce, onAddPanelOpened]);

  const filtered = artists.filter((a) => a.title.toLowerCase().includes(filterText.toLowerCase()));

  const handleLookup = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const term = lookupTerm.trim();
    if (!term) return;
    setLookupLoading(true);
    try {
      setLookupResults(await api.lookupArtists(term));
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Artist lookup failed', true);
    } finally {
      setLookupLoading(false);
    }
  };

  const handleAdd = async (candidate: Partial<Artist>) => {
    if (!selectedRootFolder) {
      onNotify('Add a music root folder under Settings → Media Management first', true);
      return;
    }
    try {
      const created = await api.addArtist({
        ...candidate,
        qualityProfileId: selectedProfileId || musicProfiles[0]?.id || 1,
        rootFolderPath: selectedRootFolder,
        monitored: true,
      });
      onNotify(`Added "${created.title}" to Music library`);
      setShowAddPanel(false);
      setLookupResults([]);
      setLookupTerm('');
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add artist', true);
    }
  };

  const openArtist = async (a: Artist) => {
    setActiveArtist(a);
    try {
      setAlbums(await api.getAlbums(a.id));
    } catch {
      setAlbums([]);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <h1 className="text-xl sm:text-2xl font-bold text-white flex items-center gap-2">
            <Music className="w-5 h-5 sm:w-6 sm:h-6 text-fuchsia-400 shrink-0" />
            <span className="truncate">Music</span>
          </h1>
          <p className="text-sm text-slate-400">{artists.length} artists · MusicBrainz metadata</p>
        </div>
        <button
          type="button"
          onClick={() => setShowAddPanel(true)}
          className="inline-flex items-center justify-center gap-2 w-full sm:w-auto px-3 py-2.5 sm:py-2 rounded-lg bg-fuchsia-600 hover:bg-fuchsia-500 text-white text-sm font-medium shrink-0"
        >
          <Plus className="w-4 h-4" /> Add Artist
        </button>
      </div>

      {!musicRoots.length && (
        <div className="rounded-lg border border-amber-500/30 bg-amber-950/40 px-3 py-2 text-xs text-amber-200">
          No music root folder yet. Add one with media type <span className="font-mono">music</span> in Settings →
          Media Management (e.g. <span className="font-mono">/data/Music</span>).
        </div>
      )}

      <div className="relative">
        <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
        <input
          value={filterText}
          onChange={(e) => setFilterText(e.target.value)}
          placeholder="Filter artists…"
          className="w-full pl-9 pr-3 py-2.5 rounded-lg bg-slate-900 border border-slate-700 text-slate-100 text-sm"
        />
      </div>

      <div className="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
        {filtered.map((a) => (
          <button
            key={a.id}
            type="button"
            onClick={() => void openArtist(a)}
            className="text-left p-4 rounded-xl bg-slate-900/80 border border-slate-800 hover:border-fuchsia-500/40 min-h-[4.5rem]"
          >
            <div className="font-semibold text-white truncate">{a.title}</div>
            <div className="text-xs text-slate-500 mt-1 truncate">{a.disambiguation || a.foreignArtistId}</div>
          </button>
        ))}
        {filtered.length === 0 && (
          <p className="text-slate-500 text-sm col-span-full">No artists yet. Add one from MusicBrainz.</p>
        )}
      </div>

      {activeArtist && (
        <div className="fixed inset-0 z-40 flex items-end sm:items-center justify-center bg-black/70 p-0 sm:p-4">
          <div className="w-full sm:max-w-2xl max-h-[92vh] sm:max-h-[85vh] overflow-auto rounded-t-2xl sm:rounded-2xl bg-slate-900 border border-slate-800 p-4 space-y-3">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <h2 className="text-lg sm:text-xl font-bold text-white truncate">{activeArtist.title}</h2>
                <p className="text-xs text-slate-400">{albums.length} albums</p>
              </div>
              <button
                type="button"
                onClick={() => setActiveArtist(null)}
                className="p-2 rounded-lg bg-slate-800 text-slate-400 shrink-0"
                aria-label="Close"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <div className="flex flex-col sm:flex-row gap-2">
              <button
                type="button"
                className="px-3 py-2 rounded-lg bg-sky-600 text-white text-sm"
                onClick={() =>
                  onSearchReleases({
                    query: activeArtist.title,
                    type: 'music',
                    artistId: activeArtist.id,
                    contextLabel: activeArtist.title,
                  })
                }
              >
                Search releases
              </button>
              <button
                type="button"
                className="px-3 py-2 rounded-lg bg-rose-700/80 text-white text-sm"
                onClick={async () => {
                  const ok = await confirm({
                    title: 'Remove artist',
                    message: `Remove "${activeArtist.title}" from the Music library?`,
                    confirmLabel: 'Remove',
                  });
                  if (!ok) return;
                  await api.deleteArtist(activeArtist.id);
                  onNotify(`Removed ${activeArtist.title}`);
                  setActiveArtist(null);
                  await onRefresh();
                }}
              >
                Remove
              </button>
            </div>
            <ul className="space-y-2">
              {albums.map((al) => (
                <li
                  key={al.id}
                  className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-3 rounded-lg bg-slate-950 border border-slate-800"
                >
                  <div className="min-w-0">
                    <div className="text-sm text-white truncate flex items-center gap-2">
                      <Disc3 className="w-3.5 h-3.5 text-fuchsia-400 shrink-0" />
                      {al.title}
                    </div>
                    <div className="text-xs text-slate-500">
                      {al.albumType}
                      {al.year ? ` · ${al.year}` : ''}
                      {al.hasFile ? ' · on disk' : ''}
                    </div>
                  </div>
                  <button
                    type="button"
                    className="text-xs px-3 py-2 rounded bg-slate-800 text-sky-300 shrink-0 self-stretch sm:self-auto"
                    onClick={() =>
                      onSearchReleases({
                        query: `${activeArtist.title} ${al.title}`,
                        type: 'music',
                        artistId: activeArtist.id,
                        albumId: al.id,
                        contextLabel: `${activeArtist.title} — ${al.title}`,
                      })
                    }
                  >
                    Search
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}

      {showAddPanel && (
        <div className="fixed inset-0 z-40 flex items-end sm:items-center justify-center bg-black/70 p-0 sm:p-4">
          <div className="w-full sm:max-w-xl max-h-[92vh] overflow-auto rounded-t-2xl sm:rounded-2xl bg-slate-900 border border-slate-800 p-4 space-y-3">
            <div className="flex items-center justify-between gap-2">
              <h2 className="text-lg font-bold text-white">Add Artist</h2>
              <button
                type="button"
                onClick={() => setShowAddPanel(false)}
                className="p-2 rounded-lg bg-slate-800 text-slate-400"
                aria-label="Close"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <form onSubmit={handleLookup} className="flex flex-col sm:flex-row gap-2">
              <input
                value={lookupTerm}
                onChange={(e) => setLookupTerm(e.target.value)}
                placeholder="Search MusicBrainz…"
                className="flex-1 px-3 py-2.5 rounded-lg bg-slate-950 border border-slate-700 text-sm text-white"
              />
              <button
                type="submit"
                disabled={lookupLoading}
                className="px-4 py-2.5 rounded-lg bg-fuchsia-600 text-white text-sm shrink-0"
              >
                {lookupLoading ? '…' : 'Lookup'}
              </button>
            </form>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
              <label className="text-slate-400 block">
                Profile
                <select
                  value={selectedProfileId || ''}
                  onChange={(e) => setSelectedProfileId(Number(e.target.value))}
                  className="mt-1 w-full px-2 py-2.5 rounded bg-slate-950 border border-slate-700 text-white"
                >
                  {musicProfiles.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="text-slate-400 block">
                Root folder
                <select
                  value={selectedRootFolder}
                  onChange={(e) => setSelectedRootFolder(e.target.value)}
                  className="mt-1 w-full px-2 py-2.5 rounded bg-slate-950 border border-slate-700 text-white"
                  disabled={!musicRoots.length}
                >
                  {!musicRoots.length && <option value="">No music root configured</option>}
                  {musicRoots.map((r) => (
                    <option key={r.id} value={r.path}>
                      {r.path}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <ul className="space-y-2">
              {lookupResults.map((a) => (
                <li
                  key={a.foreignArtistId || a.title}
                  className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-3 rounded-lg bg-slate-950 border border-slate-800"
                >
                  <div className="min-w-0">
                    <div className="text-sm text-white truncate">{a.title}</div>
                    <div className="text-xs text-slate-500 truncate">{a.disambiguation || a.foreignArtistId}</div>
                  </div>
                  <button
                    type="button"
                    onClick={() => void handleAdd(a)}
                    className="px-3 py-2 rounded bg-fuchsia-600 text-white text-xs shrink-0 self-stretch sm:self-auto"
                  >
                    Add
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
      {dialog}
    </div>
  );
};
