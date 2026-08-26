// docs/site/astro.config.mjs
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { nebari } from '@nebari/starlight';
import remarkBaseLinks from './src/plugins/remark-base-links.js';

export default defineConfig({
  // base defaults to '/' for local dev; override via BASE when deployed.
  // Production: SITE=https://packs.nebari.dev BASE=/nebari-frames/
  base: process.env.BASE || '/',
  site: process.env.SITE,
  integrations: [
    starlight({
      title: 'Nebari Frames',
      description:
        'Registry and exchange for Frames: scoped, text-based context artifacts for AI conversations.',
      // Shared Nebari identity (brand colors, fonts, logo, favicon, footer, and
      // GitHub social link) comes from the @nebari/starlight theme plugin. The
      // header logo returns users to the pack catalog and the GitHub icon
      // opens this pack's repository.
      plugins: [
        nebari({
          logoHref: 'https://packs.nebari.dev/',
          githubHref: 'https://github.com/nebari-dev/nebari-frames',
        }),
      ],
      sidebar: [
        {
          label: 'Documentation',
          items: [
            { label: 'Quickstart', slug: 'quickstart' },
            { label: 'Installation', slug: 'installation' },
            { label: 'Local Development', slug: 'local-development' },
            { label: 'Troubleshooting', slug: 'troubleshooting' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Configuration', slug: 'configuration' },
            { label: 'Architecture', slug: 'architecture' },
            { label: 'CLI Reference', items: [{ autogenerate: { directory: 'reference/cli' } }] },
            { label: 'CI/CD and Releasing', slug: 'ci-cd-releasing' },
          ],
        },
      ],
    }),
  ],
  markdown: {
    // Root-absolute links in the docs content (e.g. "/installation/") need the
    // base prefix so they still resolve once deployed under BASE=/nebari-frames/.
    remarkPlugins: [[remarkBaseLinks, { base: process.env.BASE || '/' }]],
  },
});
