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
      plugins: [nebari()],
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/nebari-dev/nebari-frames' },
      ],
      // The nebari() plugin above always prepends its own default GitHub
      // link (the org, not this repo) ahead of `social`; this override
      // renders the deduplicated, repo-specific list instead. See the
      // component for details.
      components: {
        SocialIcons: './src/components/SocialIcons.astro',
      },
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
            { label: 'CLI Reference', autogenerate: { directory: 'reference/cli' } },
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
