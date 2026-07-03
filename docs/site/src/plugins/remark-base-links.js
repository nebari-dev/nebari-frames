// Prefixes the Astro `base` path onto root-absolute markdown links and images.
// No-op when base is '/'. Leaves external (has scheme), protocol-relative ('//'),
// anchor-only ('#...'), and already-prefixed URLs untouched. Idempotent.
import { visit } from 'unist-util-visit';

export function prefixUrl(url, base) {
  if (!base || base === '/') return url;
  if (!url.startsWith('/')) return url; // anchors, relative, external (scheme)
  if (url.startsWith('//')) return url; // protocol-relative
  const prefix = base.replace(/\/$/, '');
  if (url.startsWith(`${prefix}/`)) return url; // already prefixed
  return `${prefix}${url}`;
}

export default function remarkBaseLinks(options) {
  const base = options?.base ?? '/';
  return (tree) => {
    visit(tree, ['link', 'image'], (node) => {
      if (typeof node.url === 'string') {
        node.url = prefixUrl(node.url, base);
      }
    });
  };
}
