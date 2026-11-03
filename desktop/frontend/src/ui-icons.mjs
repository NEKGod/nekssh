const paths={
  settings:'<path d="m9 3-.6 3-2 .9L3.8 6 2 9l2.3 2v2L2 15l1.8 3 2.6-.9 2 .9.6 3h6l.6-3 2-.9 2.6.9 1.8-3-2.3-2v-2L22 9l-1.8-3-2.6.9-2-.9-.6-3H9Z"/><circle cx="12" cy="12" r="3"/>',
  terminal:'<path d="m5 7 4 5-4 5m7 0h7"/>',
  servers:'<rect x="4" y="3" width="16" height="7" rx="2"/><rect x="4" y="14" width="16" height="7" rx="2"/><path d="M8 6.5h.01M8 17.5h.01m4-11h5m-5 11h5"/>',
  transfer:'<path d="M8 3v14m-4-4 4 4 4-4m4 8V7m-4 4 4-4 4 4"/>',
  history:'<path d="M3 11a9 9 0 1 1 2.6 7M3 5v6h6m3-4v5l3 2"/>',
  ports:'<rect x="3" y="8" width="6" height="8" rx="2"/><rect x="15" y="8" width="6" height="8" rx="2"/><path d="M9 12h6M6 5v3m12-3v3M6 16v3m12-3v3"/>',
  spark:'<path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5L12 3Z"/>',
  sidebar:'<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/>',
  activity:'<path d="M3 12h4l3-7 4 14 3-7h4"/>',
  layers:'<path d="m12 3 10 5-10 5L2 8l10-5Zm-10 9 10 5 10-5M2 16l10 5 10-5"/>',
  disk:'<rect x="3" y="5" width="18" height="14" rx="3"/><path d="M3 13h18m-5 3h.01m-4 0h.01"/>',
  folder:'<path d="M3 7V5a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z"/>',
  plus:'<path d="M12 5v14M5 12h14"/>',
  upload:'<path d="M12 16V3m-5 5 5-5 5 5M4 15v5h16v-5"/>',
  refresh:'<path d="M20 7a9 9 0 1 0 1 9M20 3v5h-5"/>',
  arrow:'<path d="M5 12h14m-5-5 5 5-5 5"/>',
  shield:'<path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3Z"/><path d="m8 12 3 3 5-6"/>'
};
export function icon(name){
  return `<svg class="ui-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name]||paths.terminal}</svg>`;
}
