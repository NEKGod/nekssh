import MarkdownIt from 'markdown-it';
import taskLists from 'markdown-it-task-lists';
import DOMPurify from 'dompurify';
import mermaid from 'mermaid';
import hljs from 'highlight.js';

mermaid.initialize({
  startOnLoad: false,
  theme: 'dark',
  securityLevel: 'loose',
  fontFamily: 'Segoe UI, Microsoft YaHei, sans-serif'
});

const markdownIt = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: true,
  highlight(code, lang) {
    if (!lang || lang === 'mermaid' || !hljs.getLanguage(lang)) return '';
    return hljs.highlight(code, {language: lang, ignoreIllegals: true}).value;
  }
}).use(taskLists, {enabled: false, label: true});

const renderFence = markdownIt.renderer.rules.fence;
markdownIt.renderer.rules.fence = (tokens, idx, options, env, self) => {
  const token = tokens[idx];
  if (token.info.trim().split(/\s+/)[0] === 'mermaid') {
    return `<div class="mermaid">${markdownIt.utils.escapeHtml(token.content)}</div>`;
  }
  return renderFence(tokens, idx, options, env, self);
};

export function renderMarkdown(source) {
  const html = markdownIt.render(String(source || ''));
  return DOMPurify.sanitize(html);
}

function headingAnchor(text, used) {
  let slug = text.trim().toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9\u4e00-\u9fff-]/g, '');
  if (!slug) slug = 'section';
  let unique = slug;
  let number = 2;
  while (used.has(unique)) unique = `${slug}-${number++}`;
  used.add(unique);
  return unique;
}

export function collectMarkdownOutline(root) {
  const used = new Set();
  return [...root.querySelectorAll('h1,h2,h3,h4,h5,h6')].map(heading => {
    const text = heading.textContent.trim();
    if (!heading.id) heading.id = headingAnchor(text, used);
    return {level: Number(heading.tagName.slice(1)), id: heading.id, text};
  });
}

export async function renderMermaidBlocks(root) {
  const nodes = [...root.querySelectorAll('.mermaid:not([data-processed])')];
  for (const node of nodes) {
    try {
      await mermaid.run({nodes: [node]});
    } catch (error) {
      node.textContent = error?.message || String(error);
    }
  }
  root.querySelectorAll('.mermaid').forEach(node => {
    if (node.closest('.markdown-flow')) return;
    const figure = document.createElement('figure');
    figure.className = 'markdown-flow';
    node.replaceWith(figure);
    figure.append(node);
  });
}
