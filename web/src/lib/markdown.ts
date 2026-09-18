/**
 * 极简 Markdown 渲染器。
 *
 * 自己写而不用三方库的原因：笔记内容完全由用户自己产生，
 * 我们只需要「先整体转义、再做有限替换」这一条安全路径，
 * 避免引入完整的 HTML 解析与消毒链路。
 */

function escapeHtml(input: string): string {
  return input
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/** 只允许 http/https/mailto，阻断 javascript: 之类的协议 */
function safeHref(raw: string): string | null {
  const url = raw.trim();
  if (/^(https?:|mailto:)/i.test(url)) return url;
  if (url.startsWith('/') || url.startsWith('#')) return url;
  return null;
}

function inline(text: string): string {
  let out = text;
  out = out.replace(/`([^`]+)`/g, '<code>$1</code>');
  out = out.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  out = out.replace(/(^|[^*])\*([^*]+)\*/g, '$1<em>$2</em>');
  out = out.replace(/~~([^~]+)~~/g, '<del>$1</del>');
  out = out.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (match, label: string, href: string) => {
    const safe = safeHref(href);
    if (!safe) return match;
    return '<a href="' + safe + '" target="_blank" rel="noopener noreferrer">' + label + '</a>';
  });
  return out;
}

export function renderMarkdown(source: string): string {
  const escaped = escapeHtml(source.replace(/\r\n/g, '\n'));
  const lines = escaped.split('\n');
  const html: string[] = [];
  let listType: 'ul' | 'ol' | null = null;
  let inQuote = false;
  let paragraph: string[] = [];

  const closeList = () => {
    if (listType) {
      html.push('</' + listType + '>');
      listType = null;
    }
  };
  const closeQuote = () => {
    if (inQuote) {
      html.push('</blockquote>');
      inQuote = false;
    }
  };
  const flushParagraph = () => {
    if (paragraph.length) {
      html.push('<p>' + inline(paragraph.join('<br />')) + '</p>');
      paragraph = [];
    }
  };

  for (const line of lines) {
    if (!line.trim()) {
      flushParagraph();
      closeList();
      closeQuote();
      continue;
    }

    const heading = /^(#{1,6})\s+(.*)$/.exec(line);
    if (heading) {
      flushParagraph();
      closeList();
      closeQuote();
      const level = heading[1].length;
      html.push('<h' + level + '>' + inline(heading[2]) + '</h' + level + '>');
      continue;
    }

    if (/^(---|\*\*\*|___)\s*$/.test(line)) {
      flushParagraph();
      closeList();
      closeQuote();
      html.push('<hr />');
      continue;
    }

    const quote = /^&gt;\s?(.*)$/.exec(line);
    if (quote) {
      flushParagraph();
      closeList();
      if (!inQuote) {
        html.push('<blockquote>');
        inQuote = true;
      }
      html.push('<p>' + inline(quote[1]) + '</p>');
      continue;
    }
    closeQuote();

    const bullet = /^\s*[-*+]\s+(.*)$/.exec(line);
    if (bullet) {
      flushParagraph();
      if (listType !== 'ul') {
        closeList();
        html.push('<ul>');
        listType = 'ul';
      }
      html.push('<li>' + inline(bullet[1]) + '</li>');
      continue;
    }

    const ordered = /^\s*\d+[.)]\s+(.*)$/.exec(line);
    if (ordered) {
      flushParagraph();
      if (listType !== 'ol') {
        closeList();
        html.push('<ol>');
        listType = 'ol';
      }
      html.push('<li>' + inline(ordered[1]) + '</li>');
      continue;
    }

    closeList();
    paragraph.push(line);
  }

  flushParagraph();
  closeList();
  closeQuote();
  return html.join('\n');
}
