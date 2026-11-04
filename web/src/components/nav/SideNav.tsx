import React from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import type { NavGroup, NavView } from '../../nav';
import { groupForView } from '../../nav';

export interface SideNavProps {
  groups: NavGroup[];
  activeView: NavView;
  expanded: Record<string, boolean>;
  onToggleGroup: (groupId: string) => void;
  onSelectView: (view: NavView) => void;
  itemProps?: (index: number) => React.HTMLAttributes<HTMLElement>;
  footer?: React.ReactNode;
}

export const SideNav: React.FC<SideNavProps> = ({
  groups,
  activeView,
  expanded,
  onToggleGroup,
  onSelectView,
  itemProps,
  footer,
}) => {
  const activeGroup = groupForView(activeView);
  let rovingIndex = 0;

  return (
    <div className="flex flex-col gap-1">
      {groups.map((group) => {
        const Icon = group.icon;
        const hasChildren = Boolean(group.children?.length);
        const isExpanded = Boolean(expanded[group.id]);
        const groupActive = activeGroup === group.id;
        const leafActive = group.view === activeView;
        const parentIndex = rovingIndex++;
        const parentRoving = itemProps?.(parentIndex);

        return (
          <div key={group.id} className="space-y-0.5">
            <button
              type="button"
              {...parentRoving}
              onClick={() => {
                if (hasChildren) {
                  // Clicking the already-active open section collapses it in place.
                  if (groupActive && isExpanded) {
                    onToggleGroup(group.id);
                    return;
                  }
                  onSelectView(group.defaultChild ?? group.children![0].id);
                } else if (group.view) {
                  onSelectView(group.view);
                }
              }}
              className={`flex items-center justify-between gap-2.5 px-3 py-2.5 rounded-xl text-xs font-medium whitespace-nowrap transition w-full ${
                groupActive || leafActive
                  ? 'bg-indigo-600/15 text-white border border-indigo-500/30 shadow-sm'
                  : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/60 border border-transparent'
              }`}
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <Icon className={`w-4 h-4 shrink-0 ${groupActive || leafActive ? group.accent : 'text-slate-400'}`} />
                <span className="truncate">{group.label}</span>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                {group.badge !== undefined && (
                  <span
                    className={`text-[10px] font-mono px-1.5 py-0.5 rounded-md ${
                      groupActive || leafActive
                        ? 'bg-indigo-500/25 text-indigo-200'
                        : 'bg-slate-950 text-slate-400 border border-slate-800'
                    }`}
                  >
                    {group.badge}
                  </span>
                )}
                {hasChildren ? (
                  isExpanded ? (
                    <ChevronDown className="w-3.5 h-3.5 text-slate-500" />
                  ) : (
                    <ChevronRight className="w-3.5 h-3.5 text-slate-500" />
                  )
                ) : null}
              </div>
            </button>

            {hasChildren && isExpanded ? (
              <div className="ml-3 pl-2 border-l border-slate-800 space-y-0.5">
                {group.children!.map((child) => {
                  const ChildIcon = child.icon;
                  const childActive = activeView === child.id;
                  const childIndex = rovingIndex++;
                  const childRoving = itemProps?.(childIndex);
                  return (
                    <button
                      key={child.id}
                      type="button"
                      {...childRoving}
                      onClick={() => onSelectView(child.id)}
                      className={`flex items-center gap-2 w-full text-left px-2.5 py-2 rounded-lg text-[11px] font-medium transition ${
                        childActive
                          ? 'bg-indigo-500/20 text-white'
                          : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/50'
                      }`}
                    >
                      {ChildIcon ? (
                        <ChildIcon className={`w-3.5 h-3.5 ${childActive ? group.accent : 'text-slate-500'}`} />
                      ) : null}
                      <span>{child.label}</span>
                    </button>
                  );
                })}
              </div>
            ) : null}
          </div>
        );
      })}
      {footer}
    </div>
  );
};
