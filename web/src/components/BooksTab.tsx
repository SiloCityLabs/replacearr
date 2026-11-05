import React, { useMemo, useState } from 'react';
import { BookOpen, Plus, Search, X, Headphones } from 'lucide-react';
import type { Author, Book, Edition, QualityProfile, ReleaseSearchRequest, RootFolder } from '../types';
import { api } from '../api';
import { useConfirm } from './modals/ConfirmModal';

interface BooksTabProps {
  authors: Author[];
  books: Book[];
  editions: Edition[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onRefresh: () => Promise<void>;
  onSearchReleases: (req: ReleaseSearchRequest) => void;
  onNotify: (msg: string, isError?: boolean) => void;
  openAddPanel?: boolean;
  openAddNonce?: number;
  onAddPanelOpened?: () => void;
}

function pickFormatProfiles(profiles: QualityProfile[], format: string): QualityProfile[] {
  const matched = profiles.filter((p) => p.mediaType === format);
  return matched.length > 0 ? matched : profiles;
}

export const BooksTab: React.FC<BooksTabProps> = ({
  authors,
  books,
  editions,
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
  const [format, setFormat] = useState<'ebook' | 'audiobook'>('ebook');

  const formatRoots = useMemo(
    () => rootFolders.filter((r) => r.mediaType === format || r.mediaType === 'all'),
    [rootFolders, format]
  );
  const formatProfiles = useMemo(() => pickFormatProfiles(profiles, format), [profiles, format]);

  const [filterText, setFilterText] = useState('');
  const [showAddPanel, setShowAddPanel] = useState(false);
  const [lookupTerm, setLookupTerm] = useState('');
  const [lookupResults, setLookupResults] = useState<Book[]>([]);
  const [lookupLoading, setLookupLoading] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState(0);
  const [selectedRootFolder, setSelectedRootFolder] = useState('');
  const [activeBook, setActiveBook] = useState<Book | null>(null);

  React.useEffect(() => {
    if (openAddPanel) {
      setShowAddPanel(true);
      onAddPanelOpened?.();
    }
  }, [openAddPanel, openAddNonce, onAddPanelOpened]);

  React.useEffect(() => {
    setSelectedRootFolder(formatRoots[0]?.path || '');
    setSelectedProfileId(formatProfiles[0]?.id || 0);
  }, [format, formatRoots, formatProfiles]);

  const filteredBooks = books.filter((b) => {
    if (!b.title.toLowerCase().includes(filterText.toLowerCase())) return false;
    const eds = editions.filter((e) => e.bookId === b.id);
    if (eds.length === 0) return true;
    return eds.some((e) => e.format === format);
  });
  const editionsFor = (bookId: number) => editions.filter((e) => e.bookId === bookId);
  const authorName = (authorId: number) => authors.find((a) => a.id === authorId)?.title || 'Unknown';

  const handleLookup = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const term = lookupTerm.trim();
    if (!term) return;
    setLookupLoading(true);
    try {
      setLookupResults(await api.lookupBooks(term));
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Book lookup failed', true);
    } finally {
      setLookupLoading(false);
    }
  };

  const handleAdd = async (candidate: Book) => {
    if (!selectedRootFolder) {
      onNotify(`Add a ${format} root folder under Settings → Media Management first`, true);
      return;
    }
    try {
      const author: Partial<Author> = { title: 'Unknown Author' };
      const created = await api.addBook({
        ...candidate,
        author,
        format,
        qualityProfileId: selectedProfileId || formatProfiles[0]?.id || 1,
        rootFolderPath: selectedRootFolder,
        monitored: true,
      });
      onNotify(`Added "${created.book.title}" (${format})`);
      setShowAddPanel(false);
      setLookupResults([]);
      setLookupTerm('');
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add book', true);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <h1 className="text-xl sm:text-2xl font-bold text-white flex items-center gap-2">
              <BookOpen className="w-5 h-5 sm:w-6 sm:h-6 text-amber-400 shrink-0" />
              <span className="truncate">Books</span>
            </h1>
            <p className="text-sm text-slate-400">
              {books.length} books · {authors.length} authors · OpenLibrary
            </p>
          </div>
          <button
            type="button"
            onClick={() => setShowAddPanel(true)}
            className="inline-flex items-center justify-center gap-2 w-full sm:w-auto px-3 py-2.5 sm:py-2 rounded-lg bg-amber-600 hover:bg-amber-500 text-white text-sm font-medium shrink-0"
          >
            <Plus className="w-4 h-4" /> Add Book
          </button>
        </div>

        <div className="grid grid-cols-2 rounded-lg overflow-hidden border border-slate-700 text-sm w-full sm:w-auto sm:inline-grid">
          <button
            type="button"
            onClick={() => setFormat('ebook')}
            className={`px-3 py-2.5 sm:py-1.5 ${format === 'ebook' ? 'bg-amber-600 text-white' : 'bg-slate-900 text-slate-400'}`}
          >
            Ebooks
          </button>
          <button
            type="button"
            onClick={() => setFormat('audiobook')}
            className={`px-3 py-2.5 sm:py-1.5 flex items-center justify-center gap-1 ${format === 'audiobook' ? 'bg-amber-600 text-white' : 'bg-slate-900 text-slate-400'}`}
          >
            <Headphones className="w-3.5 h-3.5" /> Audiobooks
          </button>
        </div>
      </div>

      {!formatRoots.length && (
        <div className="rounded-lg border border-amber-500/30 bg-amber-950/40 px-3 py-2 text-xs text-amber-200">
          No {format} root folder yet. Add one with media type <span className="font-mono">{format}</span> in Settings →
          Media Management (e.g.{' '}
          <span className="font-mono">{format === 'audiobook' ? '/data/Audiobooks' : '/data/Books'}</span>).
        </div>
      )}

      <div className="relative">
        <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
        <input
          value={filterText}
          onChange={(e) => setFilterText(e.target.value)}
          placeholder={`Filter ${format === 'audiobook' ? 'audiobooks' : 'ebooks'}…`}
          className="w-full pl-9 pr-3 py-2.5 rounded-lg bg-slate-900 border border-slate-700 text-slate-100 text-sm"
        />
      </div>

      <div className="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
        {filteredBooks.map((b) => (
          <button
            key={b.id}
            type="button"
            onClick={() => setActiveBook(b)}
            className="text-left p-4 rounded-xl bg-slate-900/80 border border-slate-800 hover:border-amber-500/40 min-h-[4.5rem]"
          >
            <div className="font-semibold text-white truncate">{b.title}</div>
            <div className="text-xs text-slate-500 mt-1 truncate">{authorName(b.authorId)}</div>
            <div className="text-xs text-slate-600 mt-1">
              {editionsFor(b.id)
                .map((e) => e.format)
                .join(', ') || 'no edition'}
            </div>
          </button>
        ))}
        {filteredBooks.length === 0 && (
          <p className="text-slate-500 text-sm col-span-full">
            No {format === 'audiobook' ? 'audiobooks' : 'ebooks'} yet. Add one from OpenLibrary.
          </p>
        )}
      </div>

      {activeBook && (
        <div className="fixed inset-0 z-40 flex items-end sm:items-center justify-center bg-black/70 p-0 sm:p-4">
          <div className="w-full sm:max-w-xl max-h-[92vh] overflow-auto rounded-t-2xl sm:rounded-2xl bg-slate-900 border border-slate-800 p-4 space-y-3">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <h2 className="text-lg sm:text-xl font-bold text-white truncate">{activeBook.title}</h2>
                <p className="text-xs text-slate-400">{authorName(activeBook.authorId)}</p>
              </div>
              <button
                type="button"
                onClick={() => setActiveBook(null)}
                className="p-2 rounded-lg bg-slate-800 text-slate-400 shrink-0"
                aria-label="Close"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <ul className="space-y-2">
              {editionsFor(activeBook.id).map((ed) => (
                <li
                  key={ed.id}
                  className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-3 rounded-lg bg-slate-950 border border-slate-800"
                >
                  <div className="text-sm text-white">
                    {ed.format}
                    {ed.hasFile ? ' · on disk' : ''}
                    {ed.isbn13 ? ` · ${ed.isbn13}` : ''}
                  </div>
                  <button
                    type="button"
                    className="text-xs px-3 py-2 rounded bg-slate-800 text-sky-300 self-stretch sm:self-auto"
                    onClick={() =>
                      onSearchReleases({
                        query: activeBook.title,
                        type: ed.format,
                        authorId: activeBook.authorId,
                        bookId: activeBook.id,
                        editionId: ed.id,
                        contextLabel: `${activeBook.title} (${ed.format})`,
                      })
                    }
                  >
                    Search
                  </button>
                </li>
              ))}
            </ul>
            <button
              type="button"
              className="w-full sm:w-auto px-3 py-2 rounded-lg bg-rose-700/80 text-white text-sm"
              onClick={async () => {
                const ok = await confirm({
                  title: 'Remove book',
                  message: `Remove "${activeBook.title}" from the Books library?`,
                  confirmLabel: 'Remove',
                });
                if (!ok) return;
                await api.deleteBook(activeBook.id);
                onNotify(`Removed ${activeBook.title}`);
                setActiveBook(null);
                await onRefresh();
              }}
            >
              Remove book
            </button>
          </div>
        </div>
      )}

      {showAddPanel && (
        <div className="fixed inset-0 z-40 flex items-end sm:items-center justify-center bg-black/70 p-0 sm:p-4">
          <div className="w-full sm:max-w-xl max-h-[92vh] overflow-auto rounded-t-2xl sm:rounded-2xl bg-slate-900 border border-slate-800 p-4 space-y-3">
            <div className="flex items-center justify-between gap-2">
              <h2 className="text-lg font-bold text-white">
                Add {format === 'audiobook' ? 'Audiobook' : 'Ebook'}
              </h2>
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
                placeholder="Search OpenLibrary (title, ISBN, ol:…)…"
                className="flex-1 px-3 py-2.5 rounded-lg bg-slate-950 border border-slate-700 text-sm text-white"
              />
              <button
                type="submit"
                disabled={lookupLoading}
                className="px-4 py-2.5 rounded-lg bg-amber-600 text-white text-sm shrink-0"
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
                  {formatProfiles.map((p) => (
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
                  disabled={!formatRoots.length}
                >
                  {!formatRoots.length && <option value="">No {format} root configured</option>}
                  {formatRoots.map((r) => (
                    <option key={r.id} value={r.path}>
                      {r.path}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <ul className="space-y-2">
              {lookupResults.map((b) => (
                <li
                  key={b.foreignBookId || b.title}
                  className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-3 rounded-lg bg-slate-950 border border-slate-800"
                >
                  <div className="min-w-0">
                    <div className="text-sm text-white truncate">{b.title}</div>
                    <div className="text-xs text-slate-500 truncate">{b.releaseDate || b.foreignBookId}</div>
                  </div>
                  <button
                    type="button"
                    onClick={() => void handleAdd(b)}
                    className="px-3 py-2 rounded bg-amber-600 text-white text-xs shrink-0 self-stretch sm:self-auto"
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
